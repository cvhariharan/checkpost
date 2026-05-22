package handlers

import (
	"fmt"
	"net/http"

	"github.com/cvhariharan/watcher/internal/models"
	"github.com/labstack/echo/v4"
)

func (h *Handler) HandleMachinesPagination(c echo.Context) error {
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

	page, err := h.c.PaginateNodes(c.Request().Context(), models.PageRequest{
		Page:  req.Page,
		Count: req.Count,
	})
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not get machines", err, nil)
	}

	return c.JSON(http.StatusOK, PaginateMachinesResponse{
		Machines:   page.Items,
		TotalCount: page.TotalCount,
		PageCount:  page.PageCount,
	})
}

func (h *Handler) HandleGetMachine(c echo.Context) error {
	var req GetRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusInternalServerError, "invalid request", err, nil)
	}

	if req.ID == "" {
		return wrapError(http.StatusBadRequest, "id cannot be empty", fmt.Errorf("id is empty"), nil)
	}

	node, err := h.c.GetNodeByID(c.Request().Context(), models.NodeIdentity{ID: req.ID})
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting machine %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, node)
}

func (h *Handler) HandleMachineQueries(c echo.Context) error {
	var req GetRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusInternalServerError, "invalid request", err, nil)
	}

	if req.ID == "" {
		return wrapError(http.StatusBadRequest, "id cannot be empty", fmt.Errorf("id is empty"), nil)
	}

	queries, err := h.c.ListMachineQueries(c.Request().Context(), models.NodeIdentity{ID: req.ID})
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting machine queries %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"queries": queries,
	})
}

func (h *Handler) HandleMachinePolicies(c echo.Context) error {
	var req GetRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusInternalServerError, "invalid request", err, nil)
	}

	if req.ID == "" {
		return wrapError(http.StatusBadRequest, "id cannot be empty", fmt.Errorf("id is empty"), nil)
	}

	policies, err := h.c.ListPoliciesForNode(c.Request().Context(), models.NodeIdentity{ID: req.ID})
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting machine policies %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"policies": policies,
	})
}

func (h *Handler) HandleExecuteMachineQuery(c echo.Context) error {
	var req MachineQueryRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusBadRequest, "invalid request", err, nil)
	}
	SanitizeStruct(&req)

	if req.ID == "" {
		return wrapError(http.StatusBadRequest, "id cannot be empty", fmt.Errorf("id is empty"), nil)
	}
	if req.Query == "" {
		return wrapError(http.StatusBadRequest, "query cannot be empty", fmt.Errorf("query is empty"), nil)
	}

	result, err := h.c.ExecuteMachineQuery(c.Request().Context(), models.MachineQueryRequest{
		NodeUUID: req.ID,
		Query:    req.Query,
	})
	if err != nil {
		return wrapError(http.StatusInternalServerError, "error executing machine query", err, nil)
	}

	return c.JSON(http.StatusAccepted, result)
}
