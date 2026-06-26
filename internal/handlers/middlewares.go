package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/cvhariharan/checkpost/internal/core"
	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/zerodha/simplesessions/v3"
)

// Session keys.
const (
	sessionKeyUser        = "user"
	sessionKeyMethod      = "method"
	sessionKeyIDToken     = "id_token"
	sessionKeyState       = "state"
	sessionKeyNonce       = "nonce"
	sessionKeyRedirectURL = "redirect_url"

	methodPassword = "password"
	methodOIDC     = "oidc"

	// contextUserKey is where the authenticated user is stashed for handlers.
	contextUserKey = "user"
	// contextAuthMethodKey records how the request authenticated (session|token).
	contextAuthMethodKey = "auth_method"
	// contextOwnerOnlyMachines marks a no-role owner-scoped machine list request.
	contextOwnerOnlyMachines = "owner_only_machines"

	authMethodSession = "session"
	authMethodToken   = "token"
)

// acquireSession returns the current session, creating one if none exists.
func (h *Handler) acquireSession(c echo.Context) (*simplesessions.Session, error) {
	sess, err := h.sessMgr.Acquire(c.Request().Context(), c, c)
	if errors.Is(err, simplesessions.ErrInvalidSession) {
		return h.sessMgr.NewSession(c, c) // no cookie at all
	}
	if err != nil {
		return nil, err
	}
	// Acquire defers store validation, so a stale cookie
	// yields a session whose ID no longer exists. Probe it; if invalid, mint a
	// fresh session (which also writes a new cookie) so callers can write to it.
	if _, err := sess.Get(sessionKeyUser); errors.Is(err, simplesessions.ErrInvalidSession) {
		return h.sessMgr.NewSession(c, c)
	}
	return sess, nil
}

// Authenticate guards human-facing routes: it requires a valid session, re-verifies
// OIDC tokens, and re-checks the disabled flag before allowing the request through.
func (h *Handler) Authenticate(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Bearer API tokens authenticate as the owning user; a generic 401 hides why.
		if secret, ok := bearerToken(c.Request()); ok {
			user, err := h.c.AuthenticateToken(c.Request().Context(), secret)
			if err != nil {
				return wrapError(http.StatusUnauthorized, "invalid or expired token", err, nil)
			}
			c.Set(contextUserKey, user)
			c.Set(contextAuthMethodKey, authMethodToken)
			return next(c)
		}

		sess, err := h.sessMgr.Acquire(c.Request().Context(), c, c)
		if err != nil {
			return wrapError(http.StatusUnauthorized, "authentication required", err, nil)
		}

		raw, err := sess.Get(sessionKeyUser)
		if err != nil || raw == nil {
			if errors.Is(err, simplesessions.ErrInvalidSession) {
				_ = sess.ClearCookie()
			}
			return wrapError(http.StatusUnauthorized, "authentication required", err, nil)
		}

		method, _ := sess.String(sess.Get(sessionKeyMethod))
		if method == methodOIDC {
			if h.oidc == nil {
				return wrapError(http.StatusUnauthorized, "sso is not configured", nil, nil)
			}
			rawIDToken, err := sess.Get(sessionKeyIDToken)
			if err != nil || rawIDToken == nil {
				return wrapError(http.StatusUnauthorized, "missing id token", err, nil)
			}
			token, ok := rawIDToken.(string)
			if !ok {
				return wrapError(http.StatusUnauthorized, "invalid id token", nil, nil)
			}
			if _, err := h.oidc.verifier.Verify(context.Background(), token); err != nil {
				_ = sess.Destroy()
				return wrapError(http.StatusUnauthorized, "session expired", err, nil)
			}
		}

		user, err := decodeSessionUser(raw)
		if err != nil {
			return wrapError(http.StatusUnauthorized, "invalid session", err, nil)
		}

		// Re-check the disabled flag (cheap DB lookup) so disabled users are bounced.
		dbUser, err := h.c.GetUserByUUIDRepo(c.Request().Context(), user.UUID)
		if err != nil {
			return wrapError(http.StatusUnauthorized, "user no longer exists", err, nil)
		}
		if dbUser.Disabled {
			_ = sess.Destroy()
			return wrapError(http.StatusUnauthorized, "user is disabled", nil, nil)
		}

		c.Set(contextUserKey, user)
		c.Set(contextAuthMethodKey, authMethodSession)
		return next(c)
	}
}

// bearerToken extracts the secret from an "Authorization: Bearer <secret>" header.
func bearerToken(r *http.Request) (string, bool) {
	const prefix = "bearer "
	header := r.Header.Get("Authorization")
	if len(header) < len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	secret := strings.TrimSpace(header[len(prefix):])
	if secret == "" {
		return "", false
	}
	return secret, true
}

// Authorize enforces a global (domain "*") permission check for the route.
func (h *Handler) Authorize(resource, action string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, err := h.currentUser(c)
			if err != nil {
				return wrapError(http.StatusUnauthorized, "authentication required", err, nil)
			}
			allowed, err := h.c.Can(c.Request().Context(), user.UUID, resource, action)
			if err != nil {
				return wrapError(http.StatusInternalServerError, "could not check permissions", err, nil)
			}
			if !allowed {
				return wrapError(http.StatusForbidden, "insufficient permissions", nil, nil)
			}
			return next(c)
		}
	}
}

func (h *Handler) AuthorizeMachineListView() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			_, ownerOnly, err := h.machineViewAccess(c)
			if err != nil {
				return err
			}
			c.Set(contextOwnerOnlyMachines, ownerOnly)
			return next(c)
		}
	}
}

func (h *Handler) AuthorizeOwnedMachineView() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, ownerOnly, err := h.machineViewAccess(c)
			if err != nil {
				return err
			}
			if !ownerOnly {
				return next(c)
			}
			machineID := c.Param("id")
			if _, err := uuid.Parse(machineID); err != nil {
				return wrapError(http.StatusBadRequest, "invalid machine id", err, nil)
			}
			ownsMachine, err := h.c.UserOwnsNode(c.Request().Context(), user.UUID, machineID)
			if err != nil {
				return wrapError(http.StatusInternalServerError, "could not check machine ownership", err, nil)
			}
			if !ownsMachine {
				return wrapError(http.StatusForbidden, "insufficient permissions", nil, nil)
			}
			return next(c)
		}
	}
}

func (h *Handler) AuthorizeMachineOverviewSupportView() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if _, _, err := h.machineViewAccess(c); err != nil {
				return err
			}
			return next(c)
		}
	}
}

func (h *Handler) machineViewAccess(c echo.Context) (models.SessionUser, bool, error) {
	user, err := h.currentUser(c)
	if err != nil {
		return models.SessionUser{}, false, wrapError(http.StatusUnauthorized, "authentication required", err, nil)
	}
	allowed, err := h.c.Can(c.Request().Context(), user.UUID, core.ResourceMachine, core.ActionView)
	if err != nil {
		return models.SessionUser{}, false, wrapError(http.StatusInternalServerError, "could not check permissions", err, nil)
	}
	if allowed {
		return user, false, nil
	}
	ownerOnly, err := h.c.IsNoRoleOwner(c.Request().Context(), user.UUID)
	if err != nil {
		return models.SessionUser{}, false, wrapError(http.StatusInternalServerError, "could not check owner access", err, nil)
	}
	if !ownerOnly {
		return models.SessionUser{}, false, wrapError(http.StatusForbidden, "insufficient permissions", nil, nil)
	}
	return user, true, nil
}

func ownerOnlyMachineList(c echo.Context) bool {
	ownerOnly, _ := c.Get(contextOwnerOnlyMachines).(bool)
	return ownerOnly
}

// currentUser returns the authenticated user stashed by Authenticate.
func (h *Handler) currentUser(c echo.Context) (models.SessionUser, error) {
	if v, ok := c.Get(contextUserKey).(models.SessionUser); ok {
		return v, nil
	}
	return models.SessionUser{}, errors.New("no authenticated user in context")
}

// SessionOnly rejects bearer-token requests so a leaked token can't perform
// sensitive actions like minting new tokens.
func (h *Handler) SessionOnly(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if method, _ := c.Get(contextAuthMethodKey).(string); method != authMethodSession {
			return wrapError(http.StatusForbidden, "this action requires an interactive session", nil, nil)
		}
		return next(c)
	}
}

// decodeSessionUser converts the value returned by the session store (a decoded
// JSON object) into a SessionUser.
func decodeSessionUser(raw interface{}) (models.SessionUser, error) {
	var user models.SessionUser
	b, err := json.Marshal(raw)
	if err != nil {
		return user, err
	}
	if err := json.NewDecoder(bytes.NewReader(b)).Decode(&user); err != nil {
		return user, err
	}
	if user.UUID == "" {
		return user, errors.New("session user missing uuid")
	}
	return user, nil
}
