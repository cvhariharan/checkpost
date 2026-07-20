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

func (h *Handler) HandleMachinesPagination(c echo.Context) error {
	var req MachineListRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	if req.Page > 0 {
		req.Page -= 1
	}

	if req.Count == 0 {
		req.Count = CountPerPage
	}

	listReq := models.NodeListRequest{
		Page:     req.Page,
		Count:    req.Count,
		Query:    req.Query,
		Platform: req.Platform,
		OwnerID:  req.OwnerID,
		Assigned: req.Assigned,
		SortBy:   req.SortBy,
		SortDir:  req.SortDir,
	}
	var page models.Page[models.Node]
	var err error
	if ownerOnlyMachineList(c) {
		user, userErr := h.currentUser(c)
		if userErr != nil {
			return wrapError(http.StatusUnauthorized, "authentication required", userErr, nil)
		}
		page, err = h.c.PaginateNodesForUser(c.Request().Context(), user.UUID, listReq)
	} else {
		page, err = h.c.PaginateNodes(c.Request().Context(), listReq)
	}
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
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	node, err := h.c.GetNodeByID(c.Request().Context(), models.NodeIdentity{ID: req.ID})
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting machine %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, node)
}

func (h *Handler) HandleUpdateMachine(c echo.Context) error {
	var req UpdateMachineRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	node, err := h.c.UpdateNode(c.Request().Context(), models.UpdateNode{
		UUID:        req.ID,
		DisplayName: req.DisplayName,
	})
	if err != nil {
		if errors.Is(err, core.ErrInvalidNodeDisplayName) {
			return wrapError(http.StatusBadRequest, "invalid display name", err, nil)
		}
		if errors.Is(err, sql.ErrNoRows) {
			return wrapError(http.StatusNotFound, fmt.Sprintf("machine %s not found", req.ID), err, nil)
		}
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error updating machine %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, node)
}

func (h *Handler) HandleDeleteMachine(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	if err := h.c.DeleteNode(c.Request().Context(), models.ResourceID{UUID: req.ID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return wrapError(http.StatusNotFound, fmt.Sprintf("machine %s not found", req.ID), err, nil)
		}
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error deleting machine %s", req.ID), err, nil)
	}

	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleMachineQueries(c echo.Context) error {
	var req MachineQueriesRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	if req.Page > 0 {
		req.Page -= 1
	}
	if req.Count == 0 {
		req.Count = CountPerPage
	}

	page, err := h.c.ListMachineQueries(c.Request().Context(), models.NodeIdentity{ID: req.ID}, models.PageRequest{
		Page:  req.Page,
		Count: req.Count,
	})
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting machine queries %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, MachineQueriesResponse{
		Queries:    page.Items,
		TotalCount: page.TotalCount,
		PageCount:  page.PageCount,
	})
}

func (h *Handler) HandleMachinePolicies(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	policies, err := h.c.ListPoliciesForNode(c.Request().Context(), models.NodeIdentity{ID: req.ID})
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting machine policies %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"policies": policies,
	})
}

func (h *Handler) HandleMachineGroups(c echo.Context) error {
	var req MachineGroupsRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	groups, err := h.c.ListGroupsForNode(c.Request().Context(), models.NodeIdentity{ID: req.ID})
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting machine groups %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, MachineGroupsResponse{Groups: groups})
}

func (h *Handler) HandleReplaceMachineGroups(c echo.Context) error {
	var req ReplaceMachineGroupsRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	groups, err := h.c.ReplaceGroupsForNode(c.Request().Context(), models.NodeIdentity{ID: req.ID}, req.GroupIDs)
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error updating machine groups %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, MachineGroupsResponse{Groups: groups})
}

func (h *Handler) HandleDeleteMachineQuery(c echo.Context) error {
	var req DeleteMachineQueryRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	if err := h.c.DeleteMachineQuery(c.Request().Context(), models.NodeIdentity{ID: req.ID}, models.ResourceID{UUID: req.QueryID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return wrapError(http.StatusNotFound, fmt.Sprintf("query %s not found for machine %s", req.QueryID, req.ID), err, nil)
		}
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error deleting machine query %s", req.QueryID), err, nil)
	}

	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleMachineMetrics(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	metrics, err := h.c.GetNodeMetrics(c.Request().Context(), models.NodeIdentity{ID: req.ID})
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting machine metrics %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"metrics": metrics,
	})
}

func (h *Handler) HandleExecuteMachineQuery(c echo.Context) error {
	var req MachineQueryRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	result, err := h.c.ExecuteMachineQuery(c.Request().Context(), models.MachineQueryRequest{
		NodeUUID: req.ID,
		Query:    req.Query,
	})
	if err != nil {
		if errors.Is(err, core.ErrResultsBackendDisabled) {
			return wrapError(http.StatusConflict, "results backend not configured", err, nil)
		}
		return wrapError(http.StatusInternalServerError, "error executing machine query", err, nil)
	}

	return c.JSON(http.StatusAccepted, result)
}

func (h *Handler) HandleAdHocQueryResults(c echo.Context) error {
	if c.QueryParam("format") != "" {
		return h.HandleExportAdHocResults(c)
	}
	var req AdHocResultsRequest
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

	res, err := h.c.GetAdHocQueryResults(c.Request().Context(), req.QueryID, req.Page, req.Count)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return wrapError(http.StatusNotFound, fmt.Sprintf("query %s not found", req.QueryID), err, nil)
		}
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting results for query %s", req.QueryID), err, nil)
	}

	return c.JSON(http.StatusOK, res)
}
