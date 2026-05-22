package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/cvhariharan/watcher/internal/models"
	"github.com/labstack/echo/v4"
)

func (h *Handler) HandleCreateGroup(c echo.Context) error {
	var req CreateGroupRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusBadRequest, "invalid request", err, nil)
	}
	SanitizeStruct(&req)
	if err := h.validate.Struct(req); err != nil {
		return wrapError(http.StatusBadRequest, fmt.Sprintf("invalid request: %s", formatValidationErrors(err)), err, nil)
	}

	group, err := h.c.CreateGroup(c.Request().Context(), models.CreateGroup{
		Name:        req.Title,
		Description: req.Description,
	})
	if err != nil {
		return wrapError(http.StatusInternalServerError, "error creating group", err, nil)
	}

	return c.JSON(http.StatusCreated, CreateResponse{ID: group.UUID})
}

func (h *Handler) HandleGroupsPagination(c echo.Context) error {
	var req PaginateRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusInternalServerError, "invalid request", err, nil)
	}
	if req.Page < 0 || req.Count < 0 {
		return wrapError(http.StatusInternalServerError, "invalid request, page or count per page cannot be less than 0", fmt.Errorf("page and count per page less than zero"), nil)
	}
	if req.Page > 0 {
		req.Page -= 1
	}
	if req.Count == 0 {
		req.Count = CountPerPage
	}

	page, err := h.c.PaginateGroups(c.Request().Context(), models.PageRequest{Page: req.Page, Count: req.Count})
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not get groups", err, nil)
	}

	return c.JSON(http.StatusOK, PaginateGroupsResponse{
		Groups:     page.Items,
		TotalCount: page.TotalCount,
		PageCount:  page.PageCount,
	})
}

func (h *Handler) HandleGetGroup(c echo.Context) error {
	var req GetRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusInternalServerError, "invalid request", err, nil)
	}
	if req.ID == "" {
		return wrapError(http.StatusBadRequest, "id cannot be empty", fmt.Errorf("id is empty"), nil)
	}

	group, err := h.c.GetGroup(c.Request().Context(), models.ResourceID{UUID: req.ID})
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting group %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, group)
}

func (h *Handler) HandleUpdateGroup(c echo.Context) error {
	var req UpdateGroupRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusInternalServerError, "invalid request", err, nil)
	}
	SanitizeStruct(&req)
	if req.ID == "" {
		return wrapError(http.StatusBadRequest, "id cannot be empty", fmt.Errorf("id is empty"), nil)
	}
	if err := h.validate.Struct(req); err != nil {
		return wrapError(http.StatusBadRequest, fmt.Sprintf("invalid request: %s", formatValidationErrors(err)), err, nil)
	}

	group, err := h.c.UpdateGroup(c.Request().Context(), models.UpdateGroup{
		UUID:        req.ID,
		Name:        req.Title,
		Description: req.Description,
	})
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error updating group %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, group)
}

func (h *Handler) HandleDeleteGroup(c echo.Context) error {
	var req GetRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusInternalServerError, "invalid request", err, nil)
	}
	if req.ID == "" {
		return wrapError(http.StatusBadRequest, "id cannot be empty", fmt.Errorf("id is empty"), nil)
	}

	if err := h.c.DeleteGroup(c.Request().Context(), models.ResourceID{UUID: req.ID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return wrapError(http.StatusNotFound, fmt.Sprintf("group %s not found", req.ID), err, nil)
		}
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error deleting group %s", req.ID), err, nil)
	}

	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleGroupMachines(c echo.Context) error {
	var req GroupMachinesRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusInternalServerError, "invalid request", err, nil)
	}
	if req.ID == "" {
		return wrapError(http.StatusBadRequest, "id cannot be empty", fmt.Errorf("id is empty"), nil)
	}
	if req.Page < 0 || req.Count < 0 {
		return wrapError(http.StatusInternalServerError, "invalid request, page or count per page cannot be less than 0", fmt.Errorf("page and count per page less than zero"), nil)
	}
	if req.Page > 0 {
		req.Page -= 1
	}
	if req.Count == 0 {
		req.Count = CountPerPage
	}

	page, err := h.c.PaginateGroupMachines(c.Request().Context(), models.GroupMachinesRequest{
		GroupUUID: req.ID,
		Page:      req.Page,
		Count:     req.Count,
	})
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting group machines %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, GroupMachinesResponse{
		Machines:   page.Items,
		TotalCount: page.TotalCount,
		PageCount:  page.PageCount,
	})
}
