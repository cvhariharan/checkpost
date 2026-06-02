package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/cvhariharan/watcher/internal/models"
	"github.com/cvhariharan/watcher/internal/repo"
	"github.com/google/uuid"
)

const (
	// TokenSecretPrefix makes leaked tokens greppable by secret scanners.
	TokenSecretPrefix = "wtch_pat_"
	TokenSourceSelf   = "self"

	defaultTokenTTL       = 7 * 24 * time.Hour
	tokenLastUsedThrottle = 5 * time.Minute
	tokenSweepInterval    = time.Hour
)

var (
	ErrInvalidToken  = errors.New("invalid api token")
	ErrTokenExpired  = errors.New("api token expired")
	ErrTokenRevoked  = errors.New("api token revoked")
	ErrTokenNotFound = errors.New("api token not found")
)

func generateTokenSecret() (secret, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("read random bytes: %w", err)
	}
	secret = TokenSecretPrefix + base64.RawURLEncoding.EncodeToString(buf)
	return secret, hashTokenSecret(secret), nil
}

func hashTokenSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// IssueAPIToken mints a token and returns the plaintext secret once; ttl <= 0
// falls back to defaultTokenTTL.
func (c *Core) IssueAPIToken(ctx context.Context, userID int64, name, source string, ttl time.Duration) (models.IssuedAPIToken, error) {
	secret, hash, err := generateTokenSecret()
	if err != nil {
		return models.IssuedAPIToken{}, fmt.Errorf("generate token secret: %w", err)
	}
	if strings.TrimSpace(source) == "" {
		source = TokenSourceSelf
	}
	if ttl <= 0 {
		ttl = defaultTokenTTL
	}

	row, err := c.store.CreateAPIToken(ctx, repo.CreateAPITokenParams{
		UserID:    userID,
		Name:      strings.TrimSpace(name),
		TokenHash: hash,
		Source:    source,
		ExpiresAt: sql.NullTime{Time: time.Now().Add(ttl), Valid: true},
	})
	if err != nil {
		return models.IssuedAPIToken{}, fmt.Errorf("create api token: %w", err)
	}

	return models.IssuedAPIToken{
		APIToken: apiTokenFromRepo(row),
		Secret:   secret,
	}, nil
}

// AuthenticateToken resolves a bearer secret to its user, rejecting unknown,
// expired, revoked, or disabled-user tokens.
func (c *Core) AuthenticateToken(ctx context.Context, secret string) (models.SessionUser, error) {
	secret = strings.TrimSpace(secret)
	if !strings.HasPrefix(secret, TokenSecretPrefix) {
		return models.SessionUser{}, ErrInvalidToken
	}

	row, err := c.store.GetAPITokenByHash(ctx, hashTokenSecret(secret))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.SessionUser{}, ErrInvalidToken
		}
		return models.SessionUser{}, fmt.Errorf("get api token: %w", err)
	}
	if row.RevokedAt.Valid {
		return models.SessionUser{}, ErrTokenRevoked
	}
	if row.ExpiresAt.Valid && row.ExpiresAt.Time.Before(time.Now()) {
		return models.SessionUser{}, ErrTokenExpired
	}

	user, err := c.store.GetUserByID(ctx, row.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.SessionUser{}, ErrInvalidToken
		}
		return models.SessionUser{}, fmt.Errorf("get token user: %w", err)
	}
	if user.Disabled {
		return models.SessionUser{}, ErrUserDisabled
	}

	c.touchTokenLastUsed(ctx, row)
	return SessionUserFor(toModelUser(user)), nil
}

// touchTokenLastUsed bumps last_used_at at most once per throttle window.
func (c *Core) touchTokenLastUsed(ctx context.Context, row repo.ApiToken) {
	if row.LastUsedAt.Valid && time.Since(row.LastUsedAt.Time) < tokenLastUsedThrottle {
		return
	}
	err := c.store.TouchAPITokenLastUsed(ctx, repo.TouchAPITokenLastUsedParams{
		ID:        row.ID,
		Threshold: sql.NullTime{Time: time.Now().Add(-tokenLastUsedThrottle), Valid: true},
	})
	if err != nil {
		c.logger.Warn("could not update token last_used_at", "token_id", row.ID, "error", err)
	}
}

// ListAPITokens returns the caller's own tokens (metadata only, no secrets).
func (c *Core) ListAPITokens(ctx context.Context, userUUID string) ([]models.APIToken, error) {
	uid, err := uuid.Parse(userUUID)
	if err != nil {
		return nil, fmt.Errorf("parse user uuid: %w", err)
	}
	rows, err := c.store.ListAPITokensByUser(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("list api tokens: %w", err)
	}
	out := make([]models.APIToken, 0, len(rows))
	for _, row := range rows {
		out = append(out, apiTokenFromRepo(row))
	}
	return out, nil
}

// RevokeAPIToken revokes one of the caller's tokens, returning ErrTokenNotFound
// when it doesn't exist or isn't owned by userUUID.
func (c *Core) RevokeAPIToken(ctx context.Context, userUUID, tokenUUID string) error {
	uid, err := uuid.Parse(userUUID)
	if err != nil {
		return fmt.Errorf("parse user uuid: %w", err)
	}
	tid, err := uuid.Parse(tokenUUID)
	if err != nil {
		return ErrTokenNotFound
	}
	rows, err := c.store.RevokeAPITokenForUser(ctx, repo.RevokeAPITokenForUserParams{
		TokenUuid: tid,
		UserUuid:  uid,
	})
	if err != nil {
		return fmt.Errorf("revoke api token: %w", err)
	}
	if rows == 0 {
		return ErrTokenNotFound
	}
	return nil
}

// SweepExpiredTokens deletes expired token rows and returns the count removed.
func (c *Core) SweepExpiredTokens(ctx context.Context) (int64, error) {
	n, err := c.store.DeleteExpiredAPITokens(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete expired api tokens: %w", err)
	}
	return n, nil
}

func apiTokenFromRepo(row repo.ApiToken) models.APIToken {
	return models.APIToken{
		UUID:       row.Uuid.String(),
		Name:       row.Name,
		Source:     row.Source,
		ExpiresAt:  timePtrFromNull(row.ExpiresAt),
		LastUsedAt: timePtrFromNull(row.LastUsedAt),
		RevokedAt:  timePtrFromNull(row.RevokedAt),
		CreatedAt:  row.CreatedAt,
	}
}

// TokenSweeper periodically deletes expired API tokens.
type TokenSweeper struct {
	core   *Core
	logger *slog.Logger

	mu     sync.Mutex
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

func NewTokenSweeper(c *Core, logger *slog.Logger) *TokenSweeper {
	return &TokenSweeper{
		core:   c,
		logger: logger.WithGroup("tokens.sweeper"),
	}
}

// Start launches the background goroutine. It is a no-op if already running.
func (s *TokenSweeper) Start() {
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

// Close stops the sweeper and waits for an in-flight cycle to finish.
func (s *TokenSweeper) Close() error {
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

func (s *TokenSweeper) run(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(tokenSweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := s.core.SweepExpiredTokens(ctx)
			if err != nil {
				s.logger.Error("token sweep failed", "error", err)
				continue
			}
			if n > 0 {
				s.logger.Debug("swept expired api tokens", "deleted", n)
			}
		}
	}
}
