package handlers

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cvhariharan/watcher/internal/config"
	"github.com/cvhariharan/watcher/internal/core"
	"github.com/cvhariharan/watcher/internal/models"
	"github.com/cvhariharan/watcher/internal/repo"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func TestBearerToken(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   string
		ok     bool
	}{
		{"bearer", "Bearer wtch_pat_abc", "wtch_pat_abc", true},
		{"lowercase scheme", "bearer wtch_pat_abc", "wtch_pat_abc", true},
		{"basic", "Basic dXNlcjpwYXNz", "", false},
		{"empty header", "", "", false},
		{"scheme only", "Bearer   ", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				r.Header.Set("Authorization", tc.header)
			}
			got, ok := bearerToken(r)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("bearerToken(%q) = (%q, %v), want (%q, %v)", tc.header, got, ok, tc.want, tc.ok)
			}
		})
	}
}

// fakeTokenStore satisfies repo.Store for the AuthenticateToken path.
type fakeTokenStore struct {
	repo.Store

	byHash map[string]repo.ApiToken
	users  map[int64]repo.User
}

func (s *fakeTokenStore) CreateAPIToken(ctx context.Context, arg repo.CreateAPITokenParams) (repo.ApiToken, error) {
	row := repo.ApiToken{
		Uuid:      uuid.New(),
		UserID:    arg.UserID,
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
	return nil
}

func newTokenTestHandler(t *testing.T, store repo.Store) *Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	c, err := core.NewCore(logger, store, nil, nil, nil, config.AppConfig{})
	if err != nil {
		t.Fatalf("NewCore() error = %v", err)
	}
	return &Handler{c: c, logger: logger}
}

func TestAuthenticateBearerBranch(t *testing.T) {
	store := &fakeTokenStore{byHash: map[string]repo.ApiToken{}, users: map[int64]repo.User{}}
	userUUID := uuid.New()
	store.users[7] = repo.User{ID: 7, Uuid: userUUID, Username: "a@b.com"}
	h := newTokenTestHandler(t, store)

	issued, err := h.c.IssueAPIToken(context.Background(), 7, "cli", core.TokenSourceSelf, time.Hour)
	if err != nil {
		t.Fatalf("IssueAPIToken() error = %v", err)
	}

	t.Run("valid token sets user and method", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		req.Header.Set("Authorization", "Bearer "+issued.Secret)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		var gotUser models.SessionUser
		var gotMethod string
		next := func(c echo.Context) error {
			gotUser, _ = c.Get(contextUserKey).(models.SessionUser)
			gotMethod, _ = c.Get(contextAuthMethodKey).(string)
			return c.NoContent(http.StatusOK)
		}
		if err := h.Authenticate(next)(c); err != nil {
			t.Fatalf("Authenticate() error = %v", err)
		}
		if gotUser.UUID != userUUID.String() {
			t.Fatalf("user uuid = %q, want %q", gotUser.UUID, userUUID.String())
		}
		if gotMethod != authMethodToken {
			t.Fatalf("auth method = %q, want %q", gotMethod, authMethodToken)
		}
	})

	t.Run("invalid token is 401 and skips next", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		req.Header.Set("Authorization", "Bearer wtch_pat_bogus")
		rec := httptest.NewRecorder()

		called := false
		next := func(c echo.Context) error {
			called = true
			return nil
		}
		err := h.Authenticate(next)(e.NewContext(req, rec))
		he, ok := err.(*HTTPError)
		if !ok || he.code != http.StatusUnauthorized {
			t.Fatalf("err = %v, want 401 HTTPError", err)
		}
		if called {
			t.Fatal("next ran for an invalid token")
		}
	})
}

func TestSessionOnly(t *testing.T) {
	h := &Handler{}
	e := echo.New()

	newCtx := func(method string) echo.Context {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", nil)
		c := e.NewContext(req, httptest.NewRecorder())
		c.Set(contextAuthMethodKey, method)
		return c
	}

	t.Run("token auth is forbidden and skips next", func(t *testing.T) {
		called := false
		next := func(c echo.Context) error { called = true; return nil }
		err := h.SessionOnly(next)(newCtx(authMethodToken))
		he, ok := err.(*HTTPError)
		if !ok || he.code != http.StatusForbidden {
			t.Fatalf("err = %v, want 403 HTTPError", err)
		}
		if called {
			t.Fatal("next ran for a token-authenticated request")
		}
	})

	t.Run("session auth passes through", func(t *testing.T) {
		called := false
		next := func(c echo.Context) error { called = true; return nil }
		if err := h.SessionOnly(next)(newCtx(authMethodSession)); err != nil {
			t.Fatalf("SessionOnly() error = %v", err)
		}
		if !called {
			t.Fatal("next did not run for a session request")
		}
	})
}
