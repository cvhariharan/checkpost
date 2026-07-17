package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/labstack/echo/v4"
)

func boolOrTrue(v *bool) bool {
	if v != nil {
		return *v
	}
	return true
}

func (h *Handler) HandleCreateAlertTarget(c echo.Context) error {
	var req CreateAlertTargetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	target, err := h.c.CreateAlertTarget(c.Request().Context(), models.CreateAlertTarget{
		Name:    req.Name,
		Type:    req.Type,
		Config:  req.Config,
		Enabled: boolOrTrue(req.Enabled),
	})
	if err != nil {
		return wrapError(http.StatusBadRequest, "error creating alert target", err, nil)
	}
	return c.JSON(http.StatusCreated, CreateResponse{ID: target.UUID})
}

func (h *Handler) HandleAlertTargetsPagination(c echo.Context) error {
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
	page, err := h.c.PaginateAlertTargets(c.Request().Context(), models.PageRequest{Page: req.Page, Count: req.Count})
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not get alert targets", err, nil)
	}
	return c.JSON(http.StatusOK, PaginateAlertTargetsResponse{
		Targets: page.Items, TotalCount: page.TotalCount, PageCount: page.PageCount,
	})
}

func (h *Handler) HandleGetAlertTarget(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	target, err := h.c.GetAlertTarget(c.Request().Context(), models.ResourceID{UUID: req.ID})
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting alert target %s", req.ID), err, nil)
	}
	return c.JSON(http.StatusOK, target)
}

func (h *Handler) HandleUpdateAlertTarget(c echo.Context) error {
	var req UpdateAlertTargetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	target, err := h.c.UpdateAlertTarget(c.Request().Context(), models.UpdateAlertTarget{
		UUID:    req.ID,
		Name:    req.Name,
		Config:  req.Config,
		Enabled: boolOrTrue(req.Enabled),
	})
	if err != nil {
		return wrapError(http.StatusBadRequest, fmt.Sprintf("error updating alert target %s", req.ID), err, nil)
	}
	return c.JSON(http.StatusOK, target)
}

func (h *Handler) HandleDeleteAlertTarget(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	if err := h.c.DeleteAlertTarget(c.Request().Context(), models.ResourceID{UUID: req.ID}); err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error deleting alert target %s", req.ID), err, nil)
	}
	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleTestAlertTarget(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	if err := h.c.TestTarget(c.Request().Context(), models.ResourceID{UUID: req.ID}); err != nil {
		return wrapError(http.StatusBadGateway, "test delivery failed", err, nil)
	}
	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleCreateAlertRule(c echo.Context) error {
	var req CreateAlertRuleRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	rule, err := h.c.CreateAlertRule(c.Request().Context(), toCreateAlertRule(req))
	if err != nil {
		return wrapError(http.StatusBadRequest, "error creating alert rule", err, nil)
	}
	return c.JSON(http.StatusCreated, CreateResponse{ID: rule.UUID})
}

func (h *Handler) HandleAlertRulesPagination(c echo.Context) error {
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
	page, err := h.c.PaginateAlertRules(c.Request().Context(), models.PageRequest{Page: req.Page, Count: req.Count})
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not get alert rules", err, nil)
	}
	return c.JSON(http.StatusOK, PaginateAlertRulesResponse{
		Rules: page.Items, TotalCount: page.TotalCount, PageCount: page.PageCount,
	})
}

func (h *Handler) HandleGetAlertRule(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	rule, err := h.c.GetAlertRule(c.Request().Context(), models.ResourceID{UUID: req.ID})
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting alert rule %s", req.ID), err, nil)
	}
	return c.JSON(http.StatusOK, rule)
}

func (h *Handler) HandleAlertRuleInstances(c echo.Context) error {
	var req AlertInstancesRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	if req.Page > 0 {
		req.Page -= 1
	}
	if req.Count == 0 {
		req.Count = CountPerPage
	}
	page, err := h.c.PaginateAlertInstances(c.Request().Context(), models.AlertInstancesRequest{
		RuleUUID: req.ID,
		Status:   req.Status,
		Page:     req.Page,
		Count:    req.Count,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return wrapError(http.StatusNotFound, fmt.Sprintf("alert rule %s not found", req.ID), err, nil)
		}
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting alert instances %s", req.ID), err, nil)
	}
	return c.JSON(http.StatusOK, AlertInstancesResponse{
		Instances: page.Items, TotalCount: page.TotalCount, PageCount: page.PageCount,
	})
}

func (h *Handler) HandleUpdateAlertRule(c echo.Context) error {
	var req UpdateAlertRuleRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	rule, err := h.c.UpdateAlertRule(c.Request().Context(), models.UpdateAlertRule{
		UUID:            req.ID,
		CreateAlertRule: toCreateAlertRule(req.CreateAlertRuleRequest),
	})
	if err != nil {
		return wrapError(http.StatusBadRequest, fmt.Sprintf("error updating alert rule %s", req.ID), err, nil)
	}
	return c.JSON(http.StatusOK, rule)
}

func (h *Handler) HandleDeleteAlertRule(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	if err := h.c.DeleteAlertRule(c.Request().Context(), models.ResourceID{UUID: req.ID}); err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error deleting alert rule %s", req.ID), err, nil)
	}
	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleAlertSources(c echo.Context) error {
	return c.JSON(http.StatusOK, AlertSourcesResponse{Sources: h.c.ListAlertSources()})
}

func toCreateAlertRule(req CreateAlertRuleRequest) models.CreateAlertRule {
	return models.CreateAlertRule{
		Name:               req.Name,
		Description:        req.Description,
		Source:             req.Source,
		Params:             req.Params,
		Severity:           req.Severity,
		Enabled:            boolOrTrue(req.Enabled),
		EvaluationInterval: req.EvaluationInterval,
		For:                req.For,
		RepeatInterval:     req.RepeatInterval,
		TargetIDs:          req.TargetIDs,
	}
}
