package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/cvhariharan/watcher/internal/config"
	"github.com/cvhariharan/watcher/internal/models"
	"github.com/cvhariharan/watcher/internal/repo"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrUserDisabled is returned when a disabled account attempts to log in.
	ErrUserDisabled = errors.New("user is disabled")
	// ErrInvalidLoginMethod is returned when the wrong auth method is used.
	ErrInvalidLoginMethod = errors.New("invalid authentication method for user")
	// ErrEmailDomainNotAllowed is returned when an SSO email domain is rejected.
	ErrEmailDomainNotAllowed = errors.New("email domain is not allowed")
	// ErrUserNotProvisioned is returned when an unknown SSO user cannot be auto-created.
	ErrUserNotProvisioned = errors.New("user is not provisioned and auto-create is disabled")
	// ErrSystemUser is returned when a protected system user is modified.
	ErrSystemUser = errors.New("system user cannot be modified")
)

// EnsureAdminUser creates the built-in admin (login_type 'standard') if absent,
// otherwise refreshes its password hash so config stays the source of truth. It
// also re-asserts a global `admin` role binding so the admin can never be locked out
func (c *Core) EnsureAdminUser(ctx context.Context, username, password string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("admin username cannot be empty")
	}
	if password == "" {
		return errors.New("admin password cannot be empty")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	user, err := c.store.GetUserByUsername(ctx, username)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		user, err = c.store.CreateUser(ctx, repo.CreateUserParams{
			Username:     username,
			Name:         "Administrator",
			Email:        "",
			PasswordHash: sql.NullString{String: string(hash), Valid: true},
			LoginType:    models.LoginTypeStandard,
			Disabled:     false,
		})
		if err != nil {
			return fmt.Errorf("create admin user: %w", err)
		}
	case err != nil:
		return fmt.Errorf("get admin user: %w", err)
	default:
		if err := c.store.SetUserPasswordHashByID(ctx, repo.SetUserPasswordHashByIDParams{
			ID:           user.ID,
			PasswordHash: sql.NullString{String: string(hash), Valid: true},
		}); err != nil {
			return fmt.Errorf("update admin password: %w", err)
		}
	}

	if _, err := c.BindRole(ctx, SubjectUser, user.Uuid.String(), RoleAdmin, nil); err != nil {
		return fmt.Errorf("ensure admin role binding: %w", err)
	}
	return nil
}

// VerifyPassword compares plaintext against the user's stored bcrypt hash.
func (c *Core) VerifyPassword(user repo.User, plaintext string) error {
	if !user.PasswordHash.Valid || user.PasswordHash.String == "" {
		return ErrInvalidLoginMethod
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash.String), []byte(plaintext)); err != nil {
		return fmt.Errorf("password mismatch: %w", err)
	}
	return nil
}

// AuthenticatePassword validates a username/password login and returns the user.
// Only for admin logins
func (c *Core) AuthenticatePassword(ctx context.Context, username, password string) (models.User, error) {
	user, err := c.store.GetUserByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.User{}, ErrInvalidLoginMethod
		}
		return models.User{}, fmt.Errorf("get user: %w", err)
	}
	if user.LoginType != models.LoginTypeStandard {
		return models.User{}, ErrInvalidLoginMethod
	}
	if !strings.EqualFold(user.Username, strings.TrimSpace(c.adminUsername)) {
		return models.User{}, ErrInvalidLoginMethod
	}
	if user.Disabled {
		return models.User{}, ErrUserDisabled
	}
	if err := c.VerifyPassword(user, password); err != nil {
		return models.User{}, err
	}
	if err := c.store.SetUserLastLoginByID(ctx, user.ID); err != nil {
		return models.User{}, fmt.Errorf("set last login: %w", err)
	}
	return toModelUser(user), nil
}

// ResolveOIDCUser looks up (and optionally auto-provisions) the SSO user for the
// supplied ID-token claims, then syncs user-group membership from the groups
// claim. The username for SSO users is their email.
func (c *Core) ResolveOIDCUser(ctx context.Context, claims models.OIDCClaims, oidc config.OIDCConfig) (models.User, error) {
	email := strings.TrimSpace(strings.ToLower(claims.Email))
	if email == "" {
		return models.User{}, errors.New("oidc claims missing email")
	}
	if !emailDomainAllowed(email, oidc.AllowedDomains) {
		return models.User{}, ErrEmailDomainNotAllowed
	}

	user, err := c.store.GetUserByUsername(ctx, email)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if !oidc.AutoCreateUsers {
			return models.User{}, ErrUserNotProvisioned
		}
		user, err = c.store.CreateUser(ctx, repo.CreateUserParams{
			Username:     email,
			Name:         strings.TrimSpace(claims.Name),
			Email:        email,
			PasswordHash: sql.NullString{},
			LoginType:    models.LoginTypeOIDC,
			Disabled:     false,
		})
		if err != nil {
			return models.User{}, fmt.Errorf("auto-create oidc user: %w", err)
		}
		if role := strings.TrimSpace(oidc.DefaultRole); role != "" {
			if _, err := c.BindRole(ctx, SubjectUser, user.Uuid.String(), role, nil); err != nil {
				return models.User{}, fmt.Errorf("bind default role: %w", err)
			}
		}
	case err != nil:
		return models.User{}, fmt.Errorf("get oidc user: %w", err)
	}

	if user.Disabled {
		return models.User{}, ErrUserDisabled
	}

	if err := c.SyncUserGroupsFromClaims(ctx, user.ID, claims.Groups); err != nil {
		return models.User{}, fmt.Errorf("sync user groups: %w", err)
	}
	if err := c.store.SetUserLastLoginByID(ctx, user.ID); err != nil {
		return models.User{}, fmt.Errorf("set last login: %w", err)
	}

	return toModelUser(user), nil
}

func emailDomainAllowed(email string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return false
	}
	domain := strings.ToLower(email[at+1:])
	for _, d := range allowed {
		if strings.ToLower(strings.TrimSpace(d)) == domain {
			return true
		}
	}
	return false
}

// GetUserByUUID fetches the repo user row (used by middleware to re-check disabled).
func (c *Core) GetUserByUUIDRepo(ctx context.Context, userUUID string) (repo.User, error) {
	uid, err := uuid.Parse(userUUID)
	if err != nil {
		return repo.User{}, fmt.Errorf("parse user uuid: %w", err)
	}
	return c.store.GetUserByUUID(ctx, uid)
}

// GetUserByUsername returns the repo user row for a username (used by handlers).
func (c *Core) GetUserByUsername(ctx context.Context, username string) (repo.User, error) {
	return c.store.GetUserByUsername(ctx, strings.TrimSpace(username))
}

func (c *Core) isSystemUser(user repo.User) bool {
	return user.LoginType == models.LoginTypeStandard && strings.EqualFold(user.Username, strings.TrimSpace(c.adminUsername))
}

// CreateUser provisions an SSO-only user. The bootstrap admin is the only
// password user and is managed through EnsureAdminUser.
func (c *Core) CreateUser(ctx context.Context, req models.CreateUser) (models.User, error) {
	email := strings.TrimSpace(strings.ToLower(req.Email))
	if email == "" {
		return models.User{}, errors.New("email cannot be empty")
	}
	user, err := c.store.CreateUser(ctx, repo.CreateUserParams{
		Username:     email,
		Name:         strings.TrimSpace(req.Name),
		Email:        email,
		PasswordHash: sql.NullString{},
		LoginType:    models.LoginTypeOIDC,
		Disabled:     false,
	})
	if err != nil {
		return models.User{}, fmt.Errorf("create user: %w", err)
	}
	return toModelUser(user), nil
}

// ListUsers returns a paginated list of users.
func (c *Core) ListUsers(ctx context.Context, req models.PageRequest) (models.Page[models.User], error) {
	page := req.Page
	countPerPage := req.Count
	if countPerPage <= 0 {
		countPerPage = 10
	}
	if page < 0 {
		page = 0
	}

	rows, err := c.store.ListUsers(ctx, repo.ListUsersParams{
		LimitCount:  int32(countPerPage),
		OffsetCount: int32(page * countPerPage),
	})
	if err != nil {
		return models.Page[models.User]{}, fmt.Errorf("list users: %w", err)
	}

	out := make([]models.User, 0, len(rows))
	totalCount := 0
	for _, row := range rows {
		out = append(out, models.User{
			ID:          row.ID,
			UUID:        row.Uuid.String(),
			Username:    row.Username,
			Name:        row.Name,
			Email:       row.Email,
			LoginType:   row.LoginType,
			Disabled:    row.Disabled,
			LastLoginAt: timePtrFromNull(row.LastLoginAt),
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
		})
		totalCount = int(row.TotalCount)
	}

	return models.Page[models.User]{
		Items:      out,
		TotalCount: totalCount,
		PageCount:  pageCountFor(totalCount, countPerPage),
	}, nil
}

// UpdateUser updates a user's profile fields and disabled flag.
func (c *Core) UpdateUser(ctx context.Context, req models.UpdateUser) (models.User, error) {
	uid, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.User{}, fmt.Errorf("parse user uuid: %w", err)
	}
	existing, err := c.store.GetUserByUUID(ctx, uid)
	if err != nil {
		return models.User{}, fmt.Errorf("get user: %w", err)
	}
	if c.isSystemUser(existing) {
		return models.User{}, ErrSystemUser
	}
	user, err := c.store.UpdateUserByUUID(ctx, repo.UpdateUserByUUIDParams{
		Uuid:     uid,
		Name:     strings.TrimSpace(req.Name),
		Email:    strings.TrimSpace(req.Email),
		Disabled: req.Disabled,
	})
	if err != nil {
		return models.User{}, fmt.Errorf("update user: %w", err)
	}
	return toModelUser(user), nil
}

// DeleteUser removes a user; its memberships and bindings cascade via FK.
func (c *Core) DeleteUser(ctx context.Context, userUUID string) error {
	uid, err := uuid.Parse(userUUID)
	if err != nil {
		return fmt.Errorf("parse user uuid: %w", err)
	}
	user, err := c.store.GetUserByUUID(ctx, uid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return fmt.Errorf("get user: %w", err)
	}
	if c.isSystemUser(user) {
		return ErrSystemUser
	}
	rows, err := c.store.DeleteUserByUUID(ctx, uid)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	// Rebuild policies so the deleted subject's grouping rows are dropped.
	return c.SyncPolicies(ctx)
}

// SessionUserFor builds the minimal session identity from a model user.
func SessionUserFor(u models.User) models.SessionUser {
	return models.SessionUser{
		UUID:      u.UUID,
		Username:  u.Username,
		Name:      u.Name,
		Email:     u.Email,
		LoginType: u.LoginType,
	}
}
