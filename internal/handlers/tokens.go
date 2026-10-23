package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/cvhariharan/checkpost/internal/core"
	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/cvhariharan/checkpost/internal/repo"
	"github.com/google/uuid"
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

	if req.Role != "" {
		allowed, err := h.c.Can(c.Request().Context(), user.UUID, core.ResourceRoleBinding, core.ActionCreate)
		if err != nil {
			return wrapError(http.StatusInternalServerError, "could not check permissions", err, nil)
		}
		if !allowed {
			return wrapError(http.StatusForbidden, "only admins can assign a role to a token", nil, nil)
		}
	}

	dbUser, err := h.c.GetUserByUUIDRepo(c.Request().Context(), user.UUID)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not load user", err, nil)
	}

	ttl := time.Duration(req.ExpiresInDays) * 24 * time.Hour
	issued, err := h.c.IssueAPIToken(c.Request().Context(), dbUser.ID, req.Name, core.TokenSourceSelf, req.Role, ttl)
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

// serviceTokenUser restricts delegated token management to service users.
func (h *Handler) serviceTokenUser(c echo.Context) (repo.User, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return repo.User{}, wrapError(http.StatusBadRequest, "invalid user id", err, nil)
	}
	user, err := h.c.GetUserByUUIDRepo(c.Request().Context(), id.String())
	if errors.Is(err, sql.ErrNoRows) {
		return repo.User{}, wrapError(http.StatusNotFound, "service account not found", err, nil)
	}
	if err != nil {
		return repo.User{}, wrapError(http.StatusInternalServerError, "could not load user", err, nil)
	}
	if user.Type != models.UserTypeService {
		return repo.User{}, wrapError(http.StatusNotFound, "service account not found", nil, nil)
	}
	return user, nil
}

func (h *Handler) HandleListUserTokens(c echo.Context) error {
	user, err := h.serviceTokenUser(c)
	if err != nil {
		return err
	}
	tokens, err := h.c.ListAPITokens(c.Request().Context(), user.Uuid.String())
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not list tokens", err, nil)
	}
	return c.JSON(http.StatusOK, tokens)
}

func (h *Handler) HandleIssueUserToken(c echo.Context) error {
	user, err := h.serviceTokenUser(c)
	if err != nil {
		return err
	}
	var req IssueTokenRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	if req.Role != "" {
		return wrapError(http.StatusBadRequest, "tokens use the service account's roles", nil, nil)
	}
	issued, err := h.c.IssueAPIToken(c.Request().Context(), user.ID, req.Name, core.TokenSourceSelf, "", time.Duration(req.ExpiresInDays)*24*time.Hour)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not issue token", err, nil)
	}
	return c.JSON(http.StatusCreated, issued)
}

func (h *Handler) HandleRevokeUserToken(c echo.Context) error {
	user, err := h.serviceTokenUser(c)
	if err != nil {
		return err
	}
	if err := h.c.RevokeAPIToken(c.Request().Context(), user.Uuid.String(), c.Param("token_id")); err != nil {
		if errors.Is(err, core.ErrTokenNotFound) {
			return wrapError(http.StatusNotFound, "token not found", err, nil)
		}
		return wrapError(http.StatusInternalServerError, "could not revoke token", err, nil)
	}
	return c.NoContent(http.StatusNoContent)
}
