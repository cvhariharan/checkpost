package core

import (
	"database/sql"
	"testing"

	"github.com/cvhariharan/watcher/internal/repo"
	"golang.org/x/crypto/bcrypt"
)

func TestVerifyPassword(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct horse"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	c := &Core{}
	user := repo.User{PasswordHash: sql.NullString{String: string(hash), Valid: true}}

	if err := c.VerifyPassword(user, "correct horse"); err != nil {
		t.Fatalf("VerifyPassword() valid = %v, want nil", err)
	}
	if err := c.VerifyPassword(user, "wrong"); err == nil {
		t.Fatal("VerifyPassword() with wrong password = nil, want error")
	}

	// SSO user (no password hash) cannot password-auth.
	sso := repo.User{PasswordHash: sql.NullString{}}
	if err := c.VerifyPassword(sso, "anything"); err == nil {
		t.Fatal("VerifyPassword() on passwordless user = nil, want error")
	}
}

func TestEmailDomainAllowed(t *testing.T) {
	cases := []struct {
		email   string
		allowed []string
		want    bool
	}{
		{"a@example.com", nil, true},
		{"a@example.com", []string{}, true},
		{"a@example.com", []string{"example.com"}, true},
		{"a@EXAMPLE.com", []string{"example.com"}, true},
		{"a@evil.com", []string{"example.com"}, false},
		{"malformed", []string{"example.com"}, false},
	}
	for _, tc := range cases {
		if got := emailDomainAllowed(tc.email, tc.allowed); got != tc.want {
			t.Fatalf("emailDomainAllowed(%q, %v) = %v, want %v", tc.email, tc.allowed, got, tc.want)
		}
	}
}
