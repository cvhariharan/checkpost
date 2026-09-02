package handlers

import (
	"errors"
	"fmt"
	"net/http"

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

// HandleGenerateEnrollmentSecret creates the anonymous enrollment secret used for
// bulk enrollment. Admin-only.
func (h *Handler) HandleGenerateEnrollmentSecret(c echo.Context) error {
	user, err := h.currentUser(c)
	if err != nil {
		return wrapError(http.StatusUnauthorized, "authentication required", err, nil)
	}

	if err := h.c.GenerateAnonymousEnrollmentSecret(c.Request().Context(), user.UUID); err != nil {
		return wrapError(http.StatusInternalServerError, "could not generate enrollment secret", err, nil)
	}
	return c.NoContent(http.StatusOK)
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
