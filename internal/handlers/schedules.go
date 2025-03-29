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
		Title:    req.Title,
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

func (h *Handler) HandleSchedulesPagination(c echo.Context) error {
	var req PaginateRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusInternalServerError, "invalid request", err)
	}

	if req.Page < 0 || req.Count < 0 {
		return wrapError(http.StatusInternalServerError, "invalid request, page or count per page cannot be less than 0", fmt.Errorf("page and count per page less than zero"))
	}

	if req.Page > 0 {
		req.Page -= 1
	}

	if req.Count == 0 {
		req.Count = CountPerPage
	}

	schedules, totalCount, pageCount, err := h.c.PaginateSchedules(c.Request().Context(), req.Page, req.Count)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not get queries", err)
	}

	return c.JSON(http.StatusOK, PaginateSchedulesResponse{
		Schedules:  schedules,
		TotalCount: totalCount,
		PageCount:  pageCount,
	})
}
