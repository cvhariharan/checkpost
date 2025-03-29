package handlers

import (
	"fmt"
	"net/http"

	"github.com/cvhariharan/watcher/internal/models"
	"github.com/labstack/echo/v4"
)

func (h *Handler) HandleCreateSchedule(c echo.Context) error {
	var req CreateScheduleRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusBadRequest, "invalid request", err)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(http.StatusBadRequest, fmt.Sprintf("invalid request: %s", formatValidationErrors(err)), err)
	}

	scheduleUUID, err := h.c.CreateSchedule(c.Request().Context(), models.Schedule{
		Interval: req.Interval,
		Removed:  req.Removed,
		Snapshot: req.Snapshot,
		Platform: req.Platform,
		Version:  req.Version,
		Shard:    req.Shard,
		Denylist: req.Denylist,
	}, req.QueryID)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "error creating schedule", err)
	}

	return c.JSON(http.StatusCreated, CreateResponse{
		ID: scheduleUUID,
	})
}
