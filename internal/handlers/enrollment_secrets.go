package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/cvhariharan/checkpost/internal/core"
	"github.com/labstack/echo/v4"
)

// HandleListEnrollmentSecrets lists the tracked enrollment secret registry.
func (h *Handler) HandleListEnrollmentSecrets(c echo.Context) error {
	secrets, err := h.c.ListEnrollmentSecrets(c.Request().Context())
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not list enrollment secrets", err, nil)
	}
	return c.JSON(http.StatusOK, map[string]any{"enrollment_secrets": secrets})
}

func (h *Handler) HandleCreateBulkEnrollmentSecret(c echo.Context) error {
	var req CreateBulkEnrollmentSecretRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	user, err := h.currentUser(c)
	if err != nil {
		return wrapError(http.StatusUnauthorized, "authentication required", err, nil)
	}

	secret, token, err := h.c.CreateBulkEnrollmentSecret(c.Request().Context(), req.Name, user.UUID)
	if err != nil {
		if errors.Is(err, core.ErrEnrollmentSecretName) {
			return wrapError(http.StatusBadRequest, "enrollment key name is required", err, nil)
		}
		return wrapError(http.StatusInternalServerError, "could not create enrollment key", err, nil)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"enrollment_secret": secret,
		"token":             token,
		"commands":          h.bulkBootstrapCommands(token, ""),
	})
}

func (h *Handler) HandleEnrollmentSecretBootstrap(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	token, err := h.c.EnrollmentSecretToken(c.Request().Context(), req.ID)
	if err != nil {
		if errors.Is(err, core.ErrEnrollmentSecretNotFound) {
			return wrapError(http.StatusNotFound, fmt.Sprintf("enrollment key %s not found", req.ID), err, nil)
		}
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting install commands for enrollment key %s", req.ID), err, nil)
	}

	groups := strings.TrimSpace(c.QueryParam("groups"))
	return c.JSON(http.StatusOK, map[string]any{
		"token":    token,
		"commands": h.bulkBootstrapCommands(token, groups),
	})
}

func (h *Handler) bulkBootstrapCommands(token, groups string) map[string]string {
	checkpostURL, _, _ := bootstrapURLState(h.cfg.RootURL)
	return map[string]string{
		"linux":   bulkBootstrapCommand(checkpostURL, "linux", token, groups),
		"macos":   bulkBootstrapCommand(checkpostURL, "macos", token, groups),
		"windows": bulkBootstrapCommand(checkpostURL, "windows", token, groups),
	}
}

// HandleRevokeEnrollmentSecret revokes a secret by UUID, blocking future enrolls.
func (h *Handler) HandleRevokeEnrollmentSecret(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	user, err := h.currentUser(c)
	if err != nil {
		return wrapError(http.StatusUnauthorized, "authentication required", err, nil)
	}

	if err := h.c.RevokeEnrollmentSecret(c.Request().Context(), req.ID, user.UUID); err != nil {
		if errors.Is(err, core.ErrEnrollmentSecretNotFound) {
			return wrapError(http.StatusNotFound, fmt.Sprintf("enrollment secret %s not found or already revoked", req.ID), err, nil)
		}
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error revoking enrollment secret %s", req.ID), err, nil)
	}
	return c.NoContent(http.StatusOK)
}

// HandleEnrollmentSecretMachines lists the machines enrolled with a secret.
func (h *Handler) HandleEnrollmentSecretMachines(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	machines, err := h.c.ListMachinesForEnrollmentSecret(c.Request().Context(), req.ID)
	if err != nil {
		if errors.Is(err, core.ErrEnrollmentSecretNotFound) {
			return wrapError(http.StatusNotFound, fmt.Sprintf("enrollment secret %s not found", req.ID), err, nil)
		}
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting machines for enrollment secret %s", req.ID), err, nil)
	}
	return c.JSON(http.StatusOK, map[string]any{"machines": machines})
}
