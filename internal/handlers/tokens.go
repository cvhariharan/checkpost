package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/cvhariharan/checkpost/internal/core"
	"github.com/labstack/echo/v4"
)

// HandleIssueToken mints a token for the current user, returning the secret once.
func (h *Handler) HandleIssueToken(c echo.Context) error {
	user, err := h.currentUser(c)
	if err != nil {
		return wrapError(http.StatusUnauthorized, "authentication required", err, nil)
	}

	var req IssueTokenRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	dbUser, err := h.c.GetUserByUUIDRepo(c.Request().Context(), user.UUID)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not load user", err, nil)
	}

	ttl := time.Duration(req.ExpiresInDays) * 24 * time.Hour
	issued, err := h.c.IssueAPIToken(c.Request().Context(), dbUser.ID, req.Name, core.TokenSourceSelf, ttl)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not issue token", err, nil)
	}

	return c.JSON(http.StatusCreated, issued)
}

// HandleListTokens lists the current user's tokens (metadata only, no secrets).
func (h *Handler) HandleListTokens(c echo.Context) error {
	user, err := h.currentUser(c)
	if err != nil {
		return wrapError(http.StatusUnauthorized, "authentication required", err, nil)
	}

	tokens, err := h.c.ListAPITokens(c.Request().Context(), user.UUID)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not list tokens", err, nil)
	}
	return c.JSON(http.StatusOK, tokens)
}

// HandleRevokeToken revokes one of the current user's tokens by token UUID.
func (h *Handler) HandleRevokeToken(c echo.Context) error {
	user, err := h.currentUser(c)
	if err != nil {
		return wrapError(http.StatusUnauthorized, "authentication required", err, nil)
	}

	if err := h.c.RevokeAPIToken(c.Request().Context(), user.UUID, c.Param("id")); err != nil {
		if errors.Is(err, core.ErrTokenNotFound) {
			return wrapError(http.StatusNotFound, "token not found", err, nil)
		}
		return wrapError(http.StatusInternalServerError, "could not revoke token", err, nil)
	}
	return c.NoContent(http.StatusNoContent)
}
