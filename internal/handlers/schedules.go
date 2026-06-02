package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/cvhariharan/checkpost/internal/core"
	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/labstack/echo/v4"
)

func (h *Handler) HandleCreateSchedule(c echo.Context) error {
	var req CreateScheduleRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	sched, err := h.c.CreateSchedule(c.Request().Context(), models.CreateSchedule{
		Name:            req.Title,
		SQL:             req.Query,
		Description:     req.Description,
		IntervalSeconds: req.Interval,
		Removed:         req.Removed,
		Snapshot:        req.Snapshot,
		Platform:        req.Platform,
		Version:         req.Version,
		Shard:           req.Shard,
		Denylist:        req.Denylist,
		Enabled:         true,
		GroupIDs:        req.GroupIDs,
	})
	if err != nil {
		return wrapError(http.StatusInternalServerError, "error creating schedule", err, nil)
	}

	return c.JSON(http.StatusCreated, CreateResponse{
		ID: sched.UUID,
	})
}

func (h *Handler) HandleSchedulesPagination(c echo.Context) error {
	var req PaginateRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	if req.Page > 0 {
		req.Page -= 1
	}

	if req.Count == 0 {
		req.Count = CountPerPage
	}

	page, err := h.c.PaginateSchedules(c.Request().Context(), models.PageRequest{
		Page:  req.Page,
		Count: req.Count,
	})
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not get queries", err, nil)
	}

	return c.JSON(http.StatusOK, PaginateSchedulesResponse{
		Schedules:  page.Items,
		TotalCount: page.TotalCount,
		PageCount:  page.PageCount,
	})
}

func (h *Handler) HandleGetSchedule(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	q, err := h.c.GetSchedule(c.Request().Context(), models.ResourceID{UUID: req.ID})
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting schedule %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, q)
}

func (h *Handler) HandleDeleteSchedule(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	if err := h.c.DeleteSchedule(c.Request().Context(), models.ResourceID{UUID: req.ID}); err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error deleting schedule %s", req.ID), err, nil)
	}

	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleUpdateSchedule(c echo.Context) error {
	var req UpdateScheduleRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	q, err := h.c.UpdateSchedule(c.Request().Context(), models.UpdateSchedule{
		UUID:            req.ID,
		Name:            req.Title,
		SQL:             req.Query,
		Description:     req.Description,
		IntervalSeconds: req.Interval,
		Removed:         req.Removed,
		Snapshot:        req.Snapshot,
		Platform:        req.Platform,
		Version:         req.Version,
		Shard:           req.Shard,
		Denylist:        req.Denylist,
		Enabled:         true,
		RetentionDays:   req.RetentionDays,
		GroupIDs:        req.GroupIDs,
	})
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error updating schedule %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, q)
}

func (h *Handler) HandleScheduleResults(c echo.Context) error {
	var req ScheduleResultsRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	if req.Page > 0 {
		req.Page -= 1
	}
	if req.Count == 0 {
		req.Count = CountPerPage
	}
	if req.Count > MaxResultsPerPage {
		req.Count = MaxResultsPerPage
	}

	results, err := h.c.ListScheduleResults(c.Request().Context(), models.ScheduleResultsRequest{
		ScheduleUUID: req.ID,
		Page:         req.Page,
		Count:        req.Count,
		Query:        req.Query,
	})
	if err != nil {
		if errors.Is(err, core.ErrInvalidQuery) {
			return wrapError(http.StatusBadRequest, "invalid result query", err, nil)
		}
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting results for %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, results)
}
