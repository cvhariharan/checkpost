package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/cvhariharan/watcher/internal/models"
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

	authMethodSession = "session"
	authMethodToken   = "token"
)

// acquireSession returns the current session, creating one if none exists.
func (h *Handler) acquireSession(c echo.Context) (*simplesessions.Session, error) {
	sess, err := h.sessMgr.Acquire(c.Request().Context(), c, c)
	if errors.Is(err, simplesessions.ErrInvalidSession) {
		return h.sessMgr.NewSession(c, c)
	}
	return sess, err
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
