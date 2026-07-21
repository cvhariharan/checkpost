package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/cvhariharan/checkpost/internal/core"
	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/labstack/echo/v4"
)

const rootRedirect = "/"

// isSafeRedirect allows only relative, same-host redirect targets.
func isSafeRedirect(u string) bool {
	return len(u) > 0 && u[0] == '/' && (len(u) == 1 || (u[1] != '/' && u[1] != '\\'))
}

func generateRandomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random state: %w", err)
	}
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b), nil
}

// HandleProviders reports which auth methods the login page should render.
func (h *Handler) HandleProviders(c echo.Context) error {
	resp := ProvidersResponse{Password: true}
	if h.oidc != nil {
		label := h.oidcCfg.Label
		if label == "" {
			label = "Sign in with SSO"
		}
		resp.SSO.Enabled = true
		resp.SSO.Label = label
	}
	return c.JSON(http.StatusOK, resp)
}

// HandleLogin authenticates a username/password login and establishes a session.
func (h *Handler) HandleLogin(c echo.Context) error {
	var req LoginRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	sess, err := h.acquireSession(c)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not create session", err, nil)
	}

	user, err := h.c.AuthenticatePassword(c.Request().Context(), req.Username, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, core.ErrUserDisabled):
			return wrapError(http.StatusForbidden, "account is disabled", err, nil)
		case errors.Is(err, core.ErrInvalidLoginMethod):
			return wrapError(http.StatusUnauthorized, "invalid credentials", err, nil)
		default:
			return wrapError(http.StatusUnauthorized, "invalid credentials", err, nil)
		}
	}

	if err := sess.Set(sessionKeyUser, core.SessionUserFor(user)); err != nil {
		return wrapError(http.StatusInternalServerError, "could not save session", err, nil)
	}
	if err := sess.Set(sessionKeyMethod, methodPassword); err != nil {
		return wrapError(http.StatusInternalServerError, "could not save session", err, nil)
	}

	return c.NoContent(http.StatusOK)
}

// HandleOIDCLogin starts the SSO flow: it stores state+nonce in the session and
// redirects to the provider.
func (h *Handler) HandleOIDCLogin(c echo.Context) error {
	if h.oidc == nil {
		return wrapError(http.StatusNotFound, "sso is not configured", nil, nil)
	}

	sess, err := h.acquireSession(c)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not create session", err, nil)
	}

	state, err := generateRandomState()
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not generate login state", err, nil)
	}
	nonce, err := generateRandomState()
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not generate login nonce", err, nil)
	}

	if err := sess.Set(sessionKeyState, state); err != nil {
		return wrapError(http.StatusInternalServerError, "could not save session", err, nil)
	}
	if err := sess.Set(sessionKeyNonce, nonce); err != nil {
		return wrapError(http.StatusInternalServerError, "could not save session", err, nil)
	}
	if redirectURL := c.QueryParam("redirect_url"); redirectURL != "" && isSafeRedirect(redirectURL) {
		if err := sess.Set(sessionKeyRedirectURL, redirectURL); err != nil {
			return wrapError(http.StatusInternalServerError, "could not save session", err, nil)
		}
	}

	authURL := h.oidc.oauth2Config.AuthCodeURL(state, oidc.Nonce(nonce))
	return c.Redirect(http.StatusTemporaryRedirect, authURL)
}

// HandleAuthCallback completes the SSO flow: validates state/nonce, exchanges the
// code, verifies the ID token, resolves the user, and establishes a session.
func (h *Handler) HandleAuthCallback(c echo.Context) error {
	if h.oidc == nil {
		return wrapError(http.StatusNotFound, "sso is not configured", nil, nil)
	}

	sess, err := h.sessMgr.Acquire(c.Request().Context(), c, c)
	if err != nil {
		return wrapError(http.StatusBadRequest, "session does not exist", err, nil)
	}

	storedState, err := sess.String(sess.Get(sessionKeyState))
	if err != nil || storedState == "" || storedState != c.QueryParam("state") {
		return wrapError(http.StatusBadRequest, "invalid state parameter", err, nil)
	}

	token, err := h.oidc.oauth2Config.Exchange(context.Background(), c.QueryParam("code"))
	if err != nil {
		return wrapError(http.StatusBadGateway, "failed to exchange token", err, nil)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return wrapError(http.StatusBadGateway, "no id_token in token response", nil, nil)
	}

	idToken, err := h.oidc.verifier.Verify(context.Background(), rawIDToken)
	if err != nil {
		return wrapError(http.StatusBadGateway, "failed to verify id token", err, nil)
	}

	storedNonce, _ := sess.String(sess.Get(sessionKeyNonce))
	if idToken.Nonce != storedNonce {
		return wrapError(http.StatusBadRequest, "invalid nonce", nil, nil)
	}

	claims, err := h.parseOIDCClaims(idToken)
	if err != nil {
		return wrapError(http.StatusBadGateway, "failed to parse claims", err, nil)
	}

	user, err := h.c.ResolveOIDCUser(c.Request().Context(), claims, h.oidcCfg)
	if err != nil {
		switch {
		case errors.Is(err, core.ErrUserDisabled):
			return wrapError(http.StatusForbidden, "account is disabled", err, nil)
		case errors.Is(err, core.ErrEmailDomainNotAllowed):
			return wrapError(http.StatusForbidden, "email domain not allowed", err, nil)
		case errors.Is(err, core.ErrUserNotProvisioned):
			return wrapError(http.StatusForbidden, "user is not provisioned", err, nil)
		default:
			return wrapError(http.StatusInternalServerError, "could not resolve user", err, nil)
		}
	}

	if err := sess.Set(sessionKeyUser, core.SessionUserFor(user)); err != nil {
		return wrapError(http.StatusInternalServerError, "could not save session", err, nil)
	}
	if err := sess.Set(sessionKeyMethod, methodOIDC); err != nil {
		return wrapError(http.StatusInternalServerError, "could not save session", err, nil)
	}
	if err := sess.Set(sessionKeyIDToken, rawIDToken); err != nil {
		return wrapError(http.StatusInternalServerError, "could not save session", err, nil)
	}
	_ = sess.Delete(sessionKeyState, sessionKeyNonce)

	redirectTo := rootRedirect
	if saved, err := sess.String(sess.Get(sessionKeyRedirectURL)); err == nil && isSafeRedirect(saved) {
		redirectTo = saved
		_ = sess.Delete(sessionKeyRedirectURL)
	}

	return c.Redirect(http.StatusTemporaryRedirect, redirectTo)
}

// HandleLogout destroys the session and clears the cookie.
func (h *Handler) HandleLogout(c echo.Context) error {
	sess, err := h.sessMgr.Acquire(c.Request().Context(), c, c)
	if err != nil {
		return c.NoContent(http.StatusOK)
	}
	if err := sess.Destroy(); err != nil {
		return wrapError(http.StatusInternalServerError, "could not destroy session", err, nil)
	}
	return c.NoContent(http.StatusOK)
}

// HandleMe returns the current user plus their effective permission set.
func (h *Handler) HandleMe(c echo.Context) error {
	user, err := h.currentUser(c)
	if err != nil {
		return wrapError(http.StatusUnauthorized, "authentication required", err, nil)
	}

	if user.TokenRole != "" {
		perms := h.c.PermissionsForRole(user, user.TokenRole)
		return c.JSON(http.StatusOK, MeResponse{
			User:        perms.User,
			Roles:       perms.Roles,
			Permissions: perms.Permissions,
		})
	}

	perms, err := h.c.EffectivePermissions(c.Request().Context(), user.UUID)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not get permissions", err, nil)
	}
	ownerOnly, err := h.c.IsNoRoleOwner(c.Request().Context(), user.UUID)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not get owner access", err, nil)
	}

	return c.JSON(http.StatusOK, MeResponse{
		User:            perms.User,
		Roles:           perms.Roles,
		Permissions:     perms.Permissions,
		OwnerOnlyAccess: ownerOnly,
	})
}

func (h *Handler) parseOIDCClaims(idToken *oidc.IDToken) (models.OIDCClaims, error) {
	// Decode generically so the configured groups claim can be read by name.
	raw := map[string]any{}
	if err := idToken.Claims(&raw); err != nil {
		return models.OIDCClaims{}, err
	}

	claims := models.OIDCClaims{}
	if v, ok := raw["email"].(string); ok {
		claims.Email = v
	}
	if v, ok := raw["name"].(string); ok {
		claims.Name = v
	}
	if v, ok := raw[h.oidc.groupsClaim].([]any); ok {
		for _, g := range v {
			if s, ok := g.(string); ok {
				claims.Groups = append(claims.Groups, s)
			}
		}
	}
	return claims, nil
}
