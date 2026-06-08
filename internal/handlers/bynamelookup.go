package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/labstack/echo/v4"
)

// ByNameResponse is the payload of the view-gated by-name lookup endpoints used
// by `checkpost apply` to resolve a natural-key name to its UUID.
type ByNameResponse struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

func (h *Handler) HandleGetPolicyByName(c echo.Context) error {
	return h.lookupByName(c, h.c.PolicyUUIDByName)
}

func (h *Handler) HandleGetScheduleByName(c echo.Context) error {
	return h.lookupByName(c, h.c.ScheduleUUIDByName)
}

func (h *Handler) HandleGetAlertRuleByName(c echo.Context) error {
	return h.lookupByName(c, h.c.AlertRuleUUIDByName)
}

func (h *Handler) HandleGetAlertTargetByName(c echo.Context) error {
	return h.lookupByName(c, h.c.AlertTargetUUIDByName)
}

func (h *Handler) HandleGetGroupByName(c echo.Context) error {
	return h.lookupByName(c, h.c.GroupUUIDByName)
}

func (h *Handler) lookupByName(c echo.Context, fn func(context.Context, string) (models.LookupResult, error)) error {
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		return wrapError(http.StatusBadRequest, "name is required", nil, nil)
	}

	res, err := fn(c.Request().Context(), name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return wrapError(http.StatusNotFound, "resource not found", err, nil)
		}
		return wrapError(http.StatusInternalServerError, "could not look up resource by name", err, nil)
	}

	return c.JSON(http.StatusOK, ByNameResponse{UUID: res.UUID, Name: res.Name})
}
