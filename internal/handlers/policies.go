package handlers

import (
	"fmt"
	"net/http"

	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/labstack/echo/v4"
)

func (h *Handler) HandleCreatePolicy(c echo.Context) error {
	var req CreatePolicyRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	policy, err := h.c.CreatePolicy(c.Request().Context(), models.CreatePolicy{
		Name:        req.Title,
		Query:       req.Query,
		Description: req.Description,
		Resolution:  req.Resolution,
		Platform:    req.Platform,
		Enabled:     enabled,
		GroupIDs:    req.GroupIDs,
	})
	if err != nil {
		return wrapError(http.StatusInternalServerError, "error creating policy", err, nil)
	}

	return c.JSON(http.StatusCreated, CreateResponse{ID: policy.UUID})
}

func (h *Handler) HandlePoliciesPagination(c echo.Context) error {
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

	page, err := h.c.PaginatePolicies(c.Request().Context(), models.PageRequest{Page: req.Page, Count: req.Count})
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not get policies", err, nil)
	}

	return c.JSON(http.StatusOK, PaginatePoliciesResponse{
		Policies:   page.Items,
		TotalCount: page.TotalCount,
		PageCount:  page.PageCount,
	})
}

func (h *Handler) HandleGetPolicy(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	policy, err := h.c.GetPolicy(c.Request().Context(), models.ResourceID{UUID: req.ID})
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting policy %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, policy)
}

func (h *Handler) HandleUpdatePolicy(c echo.Context) error {
	var req UpdatePolicyRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	policy, err := h.c.UpdatePolicy(c.Request().Context(), models.UpdatePolicy{
		UUID:        req.ID,
		Name:        req.Title,
		Query:       req.Query,
		Description: req.Description,
		Resolution:  req.Resolution,
		Platform:    req.Platform,
		Enabled:     enabled,
		GroupIDs:    req.GroupIDs,
	})
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error updating policy %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, policy)
}

func (h *Handler) HandleDeletePolicy(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	if err := h.c.DeletePolicy(c.Request().Context(), models.ResourceID{UUID: req.ID}); err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error deleting policy %s", req.ID), err, nil)
	}

	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandlePolicyMachines(c echo.Context) error {
	var req PolicyMachinesRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	if req.Page > 0 {
		req.Page -= 1
	}
	if req.Count == 0 {
		req.Count = CountPerPage
	}

	page, err := h.c.PaginatePolicyMachines(c.Request().Context(), models.PolicyMachinesRequest{
		PolicyUUID: req.ID,
		Response:   req.Response,
		Page:       req.Page,
		Count:      req.Count,
	})
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting policy machines %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, PolicyMachinesResponse{
		Machines:   page.Items,
		TotalCount: page.TotalCount,
		PageCount:  page.PageCount,
	})
}
