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

func (h *Handler) HandleCreateSavedQuery(c echo.Context) error {
	var req SavedQueryRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	saved, err := h.c.CreateSavedQuery(c.Request().Context(), h.viewerUUID(c), savedQueryRequest(req))
	if err != nil {
		return savedQueryError(err, "error creating saved query")
	}
	return c.JSON(http.StatusCreated, saved)
}

func (h *Handler) HandleUpdateSavedQuery(c echo.Context) error {
	var req UpdateSavedQueryRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	saved, err := h.c.UpdateSavedQuery(c.Request().Context(), h.viewerUUID(c), req.ID, savedQueryRequest(req.SavedQueryRequest))
	if err != nil {
		return savedQueryError(err, fmt.Sprintf("error updating saved query %s", req.ID))
	}
	return c.JSON(http.StatusOK, saved)
}

func (h *Handler) HandleSavedQueries(c echo.Context) error {
	var req SavedQueriesListRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	if req.Page > 0 {
		req.Page -= 1
	}
	if req.Count == 0 {
		req.Count = CountPerPage
	}

	page, err := h.c.ListSavedQueries(c.Request().Context(), h.viewerUUID(c), models.PageRequest{
		Page:  req.Page,
		Count: req.Count,
		Query: req.Query,
	})
	if err != nil {
		return wrapError(http.StatusInternalServerError, "error getting saved queries", err, nil)
	}

	return c.JSON(http.StatusOK, SavedQueriesResponse{
		SavedQueries: page.Items,
		TotalCount:   page.TotalCount,
		PageCount:    page.PageCount,
	})
}

func (h *Handler) HandleGetSavedQuery(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	saved, err := h.c.GetSavedQuery(c.Request().Context(), h.viewerUUID(c), models.ResourceID{UUID: req.ID})
	if err != nil {
		return savedQueryError(err, fmt.Sprintf("error getting saved query %s", req.ID))
	}
	return c.JSON(http.StatusOK, saved)
}

func (h *Handler) HandleDeleteSavedQuery(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	if err := h.c.DeleteSavedQuery(c.Request().Context(), h.viewerUUID(c), models.ResourceID{UUID: req.ID}); err != nil {
		return savedQueryError(err, fmt.Sprintf("error deleting saved query %s", req.ID))
	}
	return c.NoContent(http.StatusOK)
}

func (h *Handler) viewerUUID(c echo.Context) string {
	if user, err := h.currentUser(c); err == nil {
		return user.UUID
	}
	return ""
}

func savedQueryRequest(req SavedQueryRequest) models.SavedQueryRequest {
	return models.SavedQueryRequest{
		Name:        req.Name,
		Description: req.Description,
		Query:       req.Query,
		Visibility:  req.Visibility,
		Targets: models.QueryTargets{
			HostIDs:   req.HostIDs,
			GroupIDs:  req.GroupIDs,
			Platforms: req.Platforms,
		},
	}
}

func savedQueryError(err error, fallback string) error {
	if errors.Is(err, core.ErrSavedQueryNameTaken) {
		return wrapError(http.StatusConflict, err.Error(), err, nil)
	}
	if errors.Is(err, core.ErrSavedQueryForbidden) {
		return wrapError(http.StatusForbidden, err.Error(), err, nil)
	}
	if errors.Is(err, core.ErrSavedQueryOwnerRequired) {
		return wrapError(http.StatusUnauthorized, err.Error(), err, nil)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return wrapError(http.StatusNotFound, "saved query not found", err, nil)
	}
	return wrapError(http.StatusInternalServerError, fallback, err, nil)
}
