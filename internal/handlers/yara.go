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

func (h *Handler) HandleYaraSignatureSources(c echo.Context) error {
	var req YaraSignatureSourcesRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	if req.Page > 0 {
		req.Page -= 1
	}
	if req.Count == 0 {
		req.Count = CountPerPage
	}

	page, err := h.c.ListYaraSignatureSources(c.Request().Context(), models.PageRequest{Page: req.Page, Count: req.Count})
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not get YARA signature sources", err, nil)
	}
	return c.JSON(http.StatusOK, models.YaraSignatureSourcesResponse{
		Sources:    page.Items,
		TotalCount: page.TotalCount,
		PageCount:  page.PageCount,
	})
}

func (h *Handler) HandleCreateYaraSignatureSource(c echo.Context) error {
	var req CreateYaraSignatureSourceRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	source, err := h.c.CreateYaraSignatureSource(c.Request().Context(), models.YaraSignatureSourceRequest{
		GroupID: req.GroupID,
		URL:     req.URL,
		Label:   req.Label,
		Enabled: enabled,
	})
	if err != nil {
		if errors.Is(err, core.ErrInvalidQuery) {
			return wrapError(http.StatusBadRequest, "invalid YARA signature source", err, nil)
		}
		return wrapError(http.StatusInternalServerError, "error creating YARA signature source", err, nil)
	}
	return c.JSON(http.StatusCreated, source)
}

func (h *Handler) HandleUpdateYaraSignatureSource(c echo.Context) error {
	var req UpdateYaraSignatureSourceRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	source, err := h.c.UpdateYaraSignatureSource(c.Request().Context(), models.YaraSignatureSourceRequest{
		UUID:    req.ID,
		GroupID: req.GroupID,
		URL:     req.URL,
		Label:   req.Label,
		Enabled: enabled,
	})
	if err != nil {
		if errors.Is(err, core.ErrInvalidQuery) {
			return wrapError(http.StatusBadRequest, "invalid YARA signature source", err, nil)
		}
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error updating YARA signature source %s", req.ID), err, nil)
	}
	return c.JSON(http.StatusOK, source)
}

func (h *Handler) HandleDeleteYaraSignatureSource(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	if err := h.c.DeleteYaraSignatureSource(c.Request().Context(), models.ResourceID{UUID: req.ID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return wrapError(http.StatusNotFound, fmt.Sprintf("YARA signature source %s not found", req.ID), err, nil)
		}
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error deleting YARA signature source %s", req.ID), err, nil)
	}
	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleCreateYaraScan(c echo.Context) error {
	var req CreateYaraScanRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	scan, err := h.c.CreateYaraScan(c.Request().Context(), models.YaraScanRequest{
		Paths:    req.Paths,
		GroupID:  req.GroupID,
		RuleURLs: req.RuleURLs,
	})
	if err != nil {
		if errors.Is(err, core.ErrInvalidQuery) {
			return wrapError(http.StatusBadRequest, "invalid YARA scan", err, nil)
		}
		return wrapError(http.StatusInternalServerError, "error creating YARA scan", err, nil)
	}
	return c.JSON(http.StatusAccepted, scan)
}

func (h *Handler) HandleYaraScans(c echo.Context) error {
	var req YaraScansRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	if req.Page > 0 {
		req.Page -= 1
	}
	if req.Count == 0 {
		req.Count = CountPerPage
	}

	page, err := h.c.ListYaraScans(c.Request().Context(), models.PageRequest{Page: req.Page, Count: req.Count})
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not get YARA scans", err, nil)
	}
	return c.JSON(http.StatusOK, models.YaraScansResponse{
		Scans:      page.Items,
		TotalCount: page.TotalCount,
		PageCount:  page.PageCount,
	})
}

func (h *Handler) HandleGetYaraScan(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	scan, err := h.c.GetYaraScan(c.Request().Context(), models.ResourceID{UUID: req.ID})
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting YARA scan %s", req.ID), err, nil)
	}
	return c.JSON(http.StatusOK, scan)
}

func (h *Handler) HandleYaraScanMatches(c echo.Context) error {
	var req YaraScanMatchesRequest
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

	page, err := h.c.ListYaraScanMatches(c.Request().Context(), models.ResourceID{UUID: req.ID}, models.PageRequest{Page: req.Page, Count: req.Count})
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting YARA scan matches %s", req.ID), err, nil)
	}
	return c.JSON(http.StatusOK, models.YaraScanMatchesResponse{
		Matches:    page.Items,
		TotalCount: page.TotalCount,
		PageCount:  page.PageCount,
	})
}

func (h *Handler) HandleYaraScanTargets(c echo.Context) error {
	var req YaraScanTargetsRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	if req.Page > 0 {
		req.Page -= 1
	}
	if req.Count == 0 {
		req.Count = MaxResultsPerPage
	}
	if req.Count > MaxResultsPerPage {
		req.Count = MaxResultsPerPage
	}

	page, err := h.c.ListYaraScanTargets(c.Request().Context(), models.ResourceID{UUID: req.ID}, models.PageRequest{Page: req.Page, Count: req.Count})
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting YARA scan targets %s", req.ID), err, nil)
	}
	return c.JSON(http.StatusOK, models.YaraScanTargetsResponse{
		Targets:    page.Items,
		TotalCount: page.TotalCount,
		PageCount:  page.PageCount,
	})
}
