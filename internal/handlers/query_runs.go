package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/cvhariharan/checkpost/internal/core"
	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/labstack/echo/v4"
)

func (h *Handler) HandleCreateQueryRun(c echo.Context) error {
	var req CreateQueryRunRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	var createdBy string
	if user, err := h.currentUser(c); err == nil {
		createdBy = user.UUID
	}

	run, err := h.c.CreateQueryRun(c.Request().Context(), models.QueryRunRequest{
		Query: req.Query,
		Targets: models.QueryTargets{
			HostIDs:   req.HostIDs,
			GroupIDs:  req.GroupIDs,
			Platforms: req.Platforms,
		},
		CreatedByUUID: createdBy,
	})
	if err != nil {
		if errors.Is(err, core.ErrNoQueryTargets) || errors.Is(err, core.ErrTooManyQueryTargets) {
			return wrapError(http.StatusBadRequest, err.Error(), err, nil)
		}
		if errors.Is(err, core.ErrResultsBackendDisabled) {
			return wrapError(http.StatusConflict, "results backend not configured", err, nil)
		}
		return wrapError(http.StatusInternalServerError, "error creating query run", err, nil)
	}

	return c.JSON(http.StatusAccepted, run)
}

func (h *Handler) HandleQueryRuns(c echo.Context) error {
	var req QueryRunsListRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	if req.Page > 0 {
		req.Page -= 1
	}
	if req.Count == 0 {
		req.Count = CountPerPage
	}

	page, err := h.c.ListQueryRuns(c.Request().Context(), models.PageRequest{
		Page:  req.Page,
		Count: req.Count,
	})
	if err != nil {
		return wrapError(http.StatusInternalServerError, "error getting query runs", err, nil)
	}

	return c.JSON(http.StatusOK, QueryRunsResponse{
		Runs:       page.Items,
		TotalCount: page.TotalCount,
		PageCount:  page.PageCount,
	})
}

func (h *Handler) HandleGetQueryRun(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	run, err := h.c.GetQueryRun(c.Request().Context(), models.ResourceID{UUID: req.ID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return wrapError(http.StatusNotFound, fmt.Sprintf("query run %s not found", req.ID), err, nil)
		}
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting query run %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, run)
}

func (h *Handler) HandleDeleteQueryRun(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	if err := h.c.DeleteQueryRun(c.Request().Context(), models.ResourceID{UUID: req.ID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return wrapError(http.StatusNotFound, fmt.Sprintf("query run %s not found", req.ID), err, nil)
		}
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error deleting query run %s", req.ID), err, nil)
	}

	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandlePreviewQueryTargets(c echo.Context) error {
	var req PreviewQueryTargetsRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	nodeIDs, err := h.c.ResolveQueryTargets(c.Request().Context(), models.QueryTargets{
		HostIDs:   req.HostIDs,
		GroupIDs:  req.GroupIDs,
		Platforms: req.Platforms,
	})
	if err != nil {
		return wrapError(http.StatusInternalServerError, "error previewing query targets", err, nil)
	}

	return c.JSON(http.StatusOK, PreviewQueryTargetsResponse{HostCount: len(nodeIDs)})
}
