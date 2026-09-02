package core

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/cvhariharan/checkpost/internal/repo"
	"github.com/google/uuid"
)

const (
	enrollmentTypeAnonymous = "anonymous"
	enrollmentTypeOwned     = "owned"
)

var ErrEnrollmentSecretNotFound = errors.New("enrollment secret not found")

func (c *Core) GenerateAnonymousEnrollmentSecret(ctx context.Context, createdByUserUUID string) error {
	if _, err := c.store.GetActiveAnonymousEnrollmentSecret(ctx); err == nil {
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("get anonymous enrollment secret: %w", err)
	}

	var createdBy sql.NullInt64
	if uid, err := uuid.Parse(createdByUserUUID); err == nil {
		if user, err := c.store.GetUserByUUID(ctx, uid); err == nil {
			createdBy = sql.NullInt64{Int64: user.ID, Valid: true}
		}
	}

	nonce, err := randomNonce()
	if err != nil {
		return err
	}
	if _, err := c.store.CreateEnrollmentSecret(ctx, repo.CreateEnrollmentSecretParams{
		SecretID:  nonce,
		Type:      enrollmentTypeAnonymous,
		CreatedBy: createdBy,
	}); err != nil {
		return fmt.Errorf("create anonymous enrollment secret: %w", err)
	}
	return nil
}

// AnonymousEnrollmentSecret returns the active anonymous secret without creating one.
func (c *Core) AnonymousEnrollmentSecret(ctx context.Context) (string, error) {
	row, err := c.store.GetActiveAnonymousEnrollmentSecret(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrEnrollmentSecretNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get anonymous enrollment secret: %w", err)
	}
	return c.serializeEnrollmentSecret(row.SecretID, uuid.Nil), nil
}

// MintOwnedEnrollmentSecret returns the caller's active owner-bound token,
// creating it on first use.
func (c *Core) MintOwnedEnrollmentSecret(ctx context.Context, ownerUserUUID uuid.UUID) (string, error) {
	if ownerUserUUID == uuid.Nil {
		return "", fmt.Errorf("owner required for owned enrollment secret")
	}

	user, err := c.store.GetUserByUUID(ctx, ownerUserUUID)
	if err != nil {
		return "", fmt.Errorf("get owner: %w", err)
	}

	owner := uuid.NullUUID{UUID: ownerUserUUID, Valid: true}
	row, err := c.store.GetActiveOwnedEnrollmentSecret(ctx, owner)
	if errors.Is(err, sql.ErrNoRows) {
		nonce, nErr := randomNonce()
		if nErr != nil {
			return "", nErr
		}
		row, err = c.store.CreateEnrollmentSecret(ctx, repo.CreateEnrollmentSecretParams{
			SecretID:      nonce,
			Type:          enrollmentTypeOwned,
			OwnerUserUuid: owner,
			CreatedBy:     sql.NullInt64{Int64: user.ID, Valid: true},
		})
		if err != nil {
			return "", fmt.Errorf("create owned enrollment secret: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("get owned enrollment secret: %w", err)
	}

	return c.serializeEnrollmentSecret(row.SecretID, ownerUserUUID), nil
}

func (c *Core) AuthorizeEnrollmentSecret(ctx context.Context, token string) (owner uuid.UUID, secretRowID int64, ok bool, err error) {
	fields, valid := c.decodeEnrollmentSecret(token)
	if !valid {
		return uuid.Nil, 0, false, nil
	}

	row, err := c.store.GetEnrollmentSecretBySecretID(ctx, fields.nonce)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, 0, false, nil
	}
	if err != nil {
		return uuid.Nil, 0, false, fmt.Errorf("lookup enrollment secret: %w", err)
	}
	if row.RevokedAt.Valid {
		return uuid.Nil, 0, false, nil
	}
	return rowOwner(row), row.ID, true, nil
}

// RevokeEnrollmentSecret revokes a secret by its UUID, attributing revokedBy to
// the acting user when resolvable.
func (c *Core) RevokeEnrollmentSecret(ctx context.Context, secretUUID, revokedByUserUUID string) error {
	sid, err := uuid.Parse(secretUUID)
	if err != nil {
		return ErrEnrollmentSecretNotFound
	}

	var revokedBy sql.NullInt64
	if uid, err := uuid.Parse(revokedByUserUUID); err == nil {
		if user, err := c.store.GetUserByUUID(ctx, uid); err == nil {
			revokedBy = sql.NullInt64{Int64: user.ID, Valid: true}
		}
	}

	rows, err := c.store.RevokeEnrollmentSecretByUUID(ctx, repo.RevokeEnrollmentSecretByUUIDParams{
		Uuid:      sid,
		RevokedBy: revokedBy,
	})
	if err != nil {
		return fmt.Errorf("revoke enrollment secret: %w", err)
	}
	if rows == 0 {
		return ErrEnrollmentSecretNotFound
	}
	return nil
}

// ListEnrollmentSecrets returns every registered secret, newest first.
func (c *Core) ListEnrollmentSecrets(ctx context.Context) ([]models.EnrollmentSecret, error) {
	rows, err := c.store.ListEnrollmentSecrets(ctx)
	if err != nil {
		return nil, fmt.Errorf("list enrollment secrets: %w", err)
	}
	out := make([]models.EnrollmentSecret, 0, len(rows))
	for _, row := range rows {
		out = append(out, toModelEnrollmentSecret(row))
	}
	return out, nil
}

// ListMachinesForEnrollmentSecret returns the machines enrolled with a secret.
func (c *Core) ListMachinesForEnrollmentSecret(ctx context.Context, secretUUID string) ([]models.Node, error) {
	sid, err := uuid.Parse(secretUUID)
	if err != nil {
		return nil, ErrEnrollmentSecretNotFound
	}
	rows, err := c.store.ListNodesByEnrollmentSecretID(ctx, sid)
	if err != nil {
		return nil, fmt.Errorf("list machines for enrollment secret: %w", err)
	}
	out := make([]models.Node, 0, len(rows))
	for _, row := range rows {
		out = append(out, toModelNode(row))
	}
	return out, nil
}

// SweepUnusedOwnedEnrollmentSecrets deletes owned secrets older than the
// configured TTL that no machine is using, returning the count removed.
func (c *Core) SweepUnusedOwnedEnrollmentSecrets(ctx context.Context) (int64, error) {
	cutoff := time.Now().Add(-c.enrollmentSecretTTL)
	n, err := c.store.DeleteUnusedOwnedEnrollmentSecrets(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete unused owned enrollment secrets: %w", err)
	}
	return n, nil
}

// EnrollmentSecretSweeper periodically deletes unused owned enrollment secrets.
type EnrollmentSecretSweeper struct {
	core   *Core
	logger *slog.Logger

	mu     sync.Mutex
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

func NewEnrollmentSecretSweeper(c *Core, logger *slog.Logger) *EnrollmentSecretSweeper {
	return &EnrollmentSecretSweeper{
		core:   c,
		logger: logger.WithGroup("enrollment.sweeper"),
	}
}

func (s *EnrollmentSecretSweeper) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.wg.Add(1)
	go s.run(ctx)
}

func (s *EnrollmentSecretSweeper) Close() error {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
	return nil
}

func (s *EnrollmentSecretSweeper) run(ctx context.Context) {
	defer s.wg.Done()
	interval := s.core.enrollmentSecretTTL
	if interval < time.Minute {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := s.core.SweepUnusedOwnedEnrollmentSecrets(ctx)
			if err != nil {
				s.logger.Error("enrollment secret sweep failed", "error", err)
				continue
			}
			if n > 0 {
				s.logger.Debug("swept unused owned enrollment secrets", "deleted", n)
			}
		}
	}
}

func rowOwner(row repo.EnrollmentSecret) uuid.UUID {
	if row.OwnerUserUuid.Valid {
		return row.OwnerUserUuid.UUID
	}
	return uuid.Nil
}

func randomNonce() ([]byte, error) {
	nonce := make([]byte, enrollmentNonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate enrollment nonce: %w", err)
	}
	return nonce, nil
}
