package core

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/cvhariharan/watcher/internal/repo"
	"github.com/google/uuid"
)

// fakeTokenStore satisfies repo.Store; only the token/user lookups are overridden.
type fakeTokenStore struct {
	repo.Store

	created []repo.CreateAPITokenParams
	byHash  map[string]repo.ApiToken
	users   map[int64]repo.User

	touched      []repo.TouchAPITokenLastUsedParams
	revokeRows   int64
	revokeParams []repo.RevokeAPITokenForUserParams
	listRows     []repo.ApiToken
	sweepRows    int64
}

func newFakeTokenStore() *fakeTokenStore {
	return &fakeTokenStore{
		byHash: map[string]repo.ApiToken{},
		users:  map[int64]repo.User{},
	}
}

func (s *fakeTokenStore) CreateAPIToken(ctx context.Context, arg repo.CreateAPITokenParams) (repo.ApiToken, error) {
	s.created = append(s.created, arg)
	row := repo.ApiToken{
		Uuid:      uuid.New(),
		UserID:    arg.UserID,
		Name:      arg.Name,
		TokenHash: arg.TokenHash,
		Source:    arg.Source,
		ExpiresAt: arg.ExpiresAt,
		CreatedAt: time.Now(),
	}
	s.byHash[arg.TokenHash] = row
	return row, nil
}

func (s *fakeTokenStore) GetAPITokenByHash(ctx context.Context, hash string) (repo.ApiToken, error) {
	row, ok := s.byHash[hash]
	if !ok {
		return repo.ApiToken{}, sql.ErrNoRows
	}
	return row, nil
}

func (s *fakeTokenStore) GetUserByID(ctx context.Context, id int64) (repo.User, error) {
	u, ok := s.users[id]
	if !ok {
		return repo.User{}, sql.ErrNoRows
	}
	return u, nil
}

func (s *fakeTokenStore) TouchAPITokenLastUsed(ctx context.Context, arg repo.TouchAPITokenLastUsedParams) error {
	s.touched = append(s.touched, arg)
	return nil
}

func (s *fakeTokenStore) ListAPITokensByUser(ctx context.Context, userUUID uuid.UUID) ([]repo.ApiToken, error) {
	return s.listRows, nil
}

func (s *fakeTokenStore) RevokeAPITokenForUser(ctx context.Context, arg repo.RevokeAPITokenForUserParams) (int64, error) {
	s.revokeParams = append(s.revokeParams, arg)
	return s.revokeRows, nil
}

func (s *fakeTokenStore) DeleteExpiredAPITokens(ctx context.Context) (int64, error) {
	return s.sweepRows, nil
}

func testCore(store repo.Store) *Core {
	return &Core{store: store, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func TestIssueAPIToken(t *testing.T) {
	store := newFakeTokenStore()
	c := testCore(store)

	issued, err := c.IssueAPIToken(context.Background(), 7, "cli@host", TokenSourceSelf, 3*24*time.Hour)
	if err != nil {
		t.Fatalf("IssueAPIToken() error = %v", err)
	}

	if !strings.HasPrefix(issued.Secret, TokenSecretPrefix) {
		t.Fatalf("secret %q missing prefix %q", issued.Secret, TokenSecretPrefix)
	}
	if len(store.created) != 1 {
		t.Fatalf("CreateAPIToken calls = %d, want 1", len(store.created))
	}
	got := store.created[0]
	if got.TokenHash != hashTokenSecret(issued.Secret) {
		t.Fatalf("stored hash = %q, want sha256(secret)", got.TokenHash)
	}
	if got.TokenHash == issued.Secret {
		t.Fatal("stored the plaintext secret instead of its hash")
	}
	if got.UserID != 7 {
		t.Fatalf("UserID = %d, want 7", got.UserID)
	}
	if !got.ExpiresAt.Valid {
		t.Fatal("ExpiresAt not set")
	}
	if want := time.Now().Add(3 * 24 * time.Hour); got.ExpiresAt.Time.Sub(want).Abs() > time.Minute {
		t.Fatalf("ExpiresAt = %v, want ~%v", got.ExpiresAt.Time, want)
	}
}

func TestIssueAPITokenDefaultTTL(t *testing.T) {
	store := newFakeTokenStore()
	c := testCore(store)

	if _, err := c.IssueAPIToken(context.Background(), 1, "", "", 0); err != nil {
		t.Fatalf("IssueAPIToken() error = %v", err)
	}
	got := store.created[0]
	if got.Source != TokenSourceSelf {
		t.Fatalf("Source = %q, want %q (default)", got.Source, TokenSourceSelf)
	}
	if want := time.Now().Add(defaultTokenTTL); got.ExpiresAt.Time.Sub(want).Abs() > time.Minute {
		t.Fatalf("ExpiresAt = %v, want default ~%v", got.ExpiresAt.Time, want)
	}
}

func TestAuthenticateToken(t *testing.T) {
	mkToken := func(store *fakeTokenStore, mutate func(*repo.ApiToken)) string {
		secret, hash, err := generateTokenSecret()
		if err != nil {
			t.Fatalf("generateTokenSecret() error = %v", err)
		}
		row := repo.ApiToken{
			Uuid:      uuid.New(),
			UserID:    42,
			TokenHash: hash,
			Source:    TokenSourceSelf,
			ExpiresAt: sql.NullTime{Time: time.Now().Add(time.Hour), Valid: true},
		}
		if mutate != nil {
			mutate(&row)
		}
		store.byHash[hash] = row
		return secret
	}

	t.Run("valid", func(t *testing.T) {
		store := newFakeTokenStore()
		store.users[42] = repo.User{ID: 42, Uuid: uuid.New(), Username: "a@b.com"}
		secret := mkToken(store, nil)

		user, err := testCore(store).AuthenticateToken(context.Background(), secret)
		if err != nil {
			t.Fatalf("AuthenticateToken() error = %v", err)
		}
		if user.UUID != store.users[42].Uuid.String() {
			t.Fatalf("user uuid = %q, want %q", user.UUID, store.users[42].Uuid.String())
		}
	})

	t.Run("malformed prefix", func(t *testing.T) {
		store := newFakeTokenStore()
		_, err := testCore(store).AuthenticateToken(context.Background(), "not-a-watcher-token")
		if !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("err = %v, want ErrInvalidToken", err)
		}
	})

	t.Run("unknown hash", func(t *testing.T) {
		store := newFakeTokenStore()
		_, err := testCore(store).AuthenticateToken(context.Background(), TokenSecretPrefix+"deadbeef")
		if !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("err = %v, want ErrInvalidToken", err)
		}
	})

	t.Run("revoked", func(t *testing.T) {
		store := newFakeTokenStore()
		store.users[42] = repo.User{ID: 42, Uuid: uuid.New()}
		secret := mkToken(store, func(r *repo.ApiToken) {
			r.RevokedAt = sql.NullTime{Time: time.Now(), Valid: true}
		})
		_, err := testCore(store).AuthenticateToken(context.Background(), secret)
		if !errors.Is(err, ErrTokenRevoked) {
			t.Fatalf("err = %v, want ErrTokenRevoked", err)
		}
	})

	t.Run("expired", func(t *testing.T) {
		store := newFakeTokenStore()
		store.users[42] = repo.User{ID: 42, Uuid: uuid.New()}
		secret := mkToken(store, func(r *repo.ApiToken) {
			r.ExpiresAt = sql.NullTime{Time: time.Now().Add(-time.Hour), Valid: true}
		})
		_, err := testCore(store).AuthenticateToken(context.Background(), secret)
		if !errors.Is(err, ErrTokenExpired) {
			t.Fatalf("err = %v, want ErrTokenExpired", err)
		}
	})

	t.Run("disabled user", func(t *testing.T) {
		store := newFakeTokenStore()
		store.users[42] = repo.User{ID: 42, Uuid: uuid.New(), Disabled: true}
		secret := mkToken(store, nil)
		_, err := testCore(store).AuthenticateToken(context.Background(), secret)
		if !errors.Is(err, ErrUserDisabled) {
			t.Fatalf("err = %v, want ErrUserDisabled", err)
		}
	})
}

func TestAuthenticateTokenLastUsedThrottle(t *testing.T) {
	seed := func(lastUsed sql.NullTime) (*fakeTokenStore, string) {
		store := newFakeTokenStore()
		store.users[42] = repo.User{ID: 42, Uuid: uuid.New()}
		secret, hash, err := generateTokenSecret()
		if err != nil {
			t.Fatalf("generateTokenSecret() error = %v", err)
		}
		store.byHash[hash] = repo.ApiToken{
			ID:         1,
			UserID:     42,
			TokenHash:  hash,
			ExpiresAt:  sql.NullTime{Time: time.Now().Add(time.Hour), Valid: true},
			LastUsedAt: lastUsed,
		}
		return store, secret
	}

	t.Run("stale bumps", func(t *testing.T) {
		store, secret := seed(sql.NullTime{Time: time.Now().Add(-2 * tokenLastUsedThrottle), Valid: true})
		if _, err := testCore(store).AuthenticateToken(context.Background(), secret); err != nil {
			t.Fatalf("AuthenticateToken() error = %v", err)
		}
		if len(store.touched) != 1 {
			t.Fatalf("touched calls = %d, want 1", len(store.touched))
		}
	})

	t.Run("recent skips", func(t *testing.T) {
		store, secret := seed(sql.NullTime{Time: time.Now(), Valid: true})
		if _, err := testCore(store).AuthenticateToken(context.Background(), secret); err != nil {
			t.Fatalf("AuthenticateToken() error = %v", err)
		}
		if len(store.touched) != 0 {
			t.Fatalf("touched calls = %d, want 0 (throttled)", len(store.touched))
		}
	})
}

func TestRevokeAPIToken(t *testing.T) {
	userUUID := uuid.New().String()
	tokenUUID := uuid.New().String()

	t.Run("owned", func(t *testing.T) {
		store := newFakeTokenStore()
		store.revokeRows = 1
		if err := testCore(store).RevokeAPIToken(context.Background(), userUUID, tokenUUID); err != nil {
			t.Fatalf("RevokeAPIToken() error = %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		store := newFakeTokenStore()
		store.revokeRows = 0
		err := testCore(store).RevokeAPIToken(context.Background(), userUUID, tokenUUID)
		if !errors.Is(err, ErrTokenNotFound) {
			t.Fatalf("err = %v, want ErrTokenNotFound", err)
		}
	})

	t.Run("malformed token uuid", func(t *testing.T) {
		store := newFakeTokenStore()
		err := testCore(store).RevokeAPIToken(context.Background(), userUUID, "not-a-uuid")
		if !errors.Is(err, ErrTokenNotFound) {
			t.Fatalf("err = %v, want ErrTokenNotFound", err)
		}
	})
}

func TestSweepExpiredTokens(t *testing.T) {
	store := newFakeTokenStore()
	store.sweepRows = 5
	n, err := testCore(store).SweepExpiredTokens(context.Background())
	if err != nil {
		t.Fatalf("SweepExpiredTokens() error = %v", err)
	}
	if n != 5 {
		t.Fatalf("deleted = %d, want 5", n)
	}
}
