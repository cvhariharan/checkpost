package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/cvhariharan/watcher/internal/models"
	"github.com/cvhariharan/watcher/internal/results"
	"github.com/labstack/echo/v4"
)

const (
	// CountPerPage used for pagination requests
	CountPerPage = 10

	// Maximum number of schedules that will be returned in osquery config
	ScheduleMax = 100
)

func (h *Handler) HandleEnrollment(c echo.Context) error {
	var req EnrollmentRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusBadRequest, "invalid enrollment request", err, EnrollmentResponse{NodeInvalid: true})
	}

	SanitizeStruct(&req)

	if req.EnrollSecret != h.cfg.EnrollmentKey {
		return wrapError(http.StatusUnauthorized, "invalid enrollment key", fmt.Errorf("enrollment key invalid"), EnrollmentResponse{NodeInvalid: true})
	}

	creds, err := h.c.EnrollNode(c.Request().Context(), req.ToNodeModel())
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not enroll node", err, EnrollmentResponse{NodeInvalid: true})
	}

	return c.JSON(http.StatusOK, EnrollmentResponse{
		NodeKey:     creds.NodeKey,
		NodeInvalid: false,
	})
}

func (h *Handler) HandleCreateQuery(c echo.Context) error {
	var req CreateQueryRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusInternalServerError, "invalid create query request", err, nil)
	}
	SanitizeStruct(&req)

	q, err := h.c.CreateQuery(c.Request().Context(), models.CreateQuery{
		Name:        req.Title,
		SQL:         req.Query,
		Description: req.Description,
	})
	if err != nil {
		return wrapError(http.StatusInternalServerError, "error creating query", err, nil)
	}

	resp := CreateResponse{
		ID: q.UUID,
	}

	return c.JSON(http.StatusCreated, resp)
}

func (h *Handler) HandleQueriesPagination(c echo.Context) error {
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

	page, err := h.c.PaginateQueries(c.Request().Context(), models.PageRequest{
		Page:  req.Page,
		Count: req.Count,
	})
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not get queries", err, nil)
	}

	return c.JSON(http.StatusOK, PaginateQueriesResponse{
		Queries:    page.Items,
		TotalCount: page.TotalCount,
		PageCount:  page.PageCount,
	})
}

func (h *Handler) HandleGetQuery(c echo.Context) error {
	var req GetRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusInternalServerError, "invalid request", err, nil)
	}

	if req.ID == "" {
		return wrapError(http.StatusBadRequest, "id cannot be empty", fmt.Errorf("id is empty"), nil)
	}

	q, err := h.c.GetQuery(c.Request().Context(), models.ResourceID{UUID: req.ID})
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting query %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, q)
}

func (h *Handler) HandleDeleteQuery(c echo.Context) error {
	var req GetRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusInternalServerError, "invalid request", err, nil)
	}

	if req.ID == "" {
		return wrapError(http.StatusBadRequest, "id cannot be empty", fmt.Errorf("id is empty"), nil)
	}

	if err := h.c.DeleteQuery(c.Request().Context(), models.ResourceID{UUID: req.ID}); err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error deleting query %s", req.ID), err, nil)
	}

	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleUpdateQuery(c echo.Context) error {
	var req UpdateQueryRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusInternalServerError, "invalid request", err, nil)
	}

	if req.ID == "" {
		return wrapError(http.StatusBadRequest, "id cannot be empty", fmt.Errorf("id is empty"), nil)
	}

	q, err := h.c.UpdateQuery(c.Request().Context(), models.UpdateQuery{
		UUID:        req.ID,
		Name:        req.Title,
		SQL:         req.Query,
		Description: req.Description,
	})
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error updating query %s", req.ID), err, nil)
	}

	return c.JSON(http.StatusOK, q)
}

func (h *Handler) HandleOSQueryConfig(c echo.Context) error {
	var req ConfigRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusInternalServerError, "invalid request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(http.StatusBadRequest, fmt.Sprintf("invalid request: %s", formatValidationErrors(err)), err, nil)
	}

	s, err := h.c.ListEnabledSchedulesForNode(c.Request().Context(), models.NodeKeyRequest{NodeKey: req.NodeKey}, ScheduleMax)
	if err != nil {
		return wrapError(http.StatusBadRequest, "error getting schedules for node", err, EnrollmentResponse{NodeInvalid: true})
	}

	h.logger.Debug("config", "response", s)

	osc := OSQueryConfigResponse{
		Schedule: make(map[string]ScheduleConfig),
	}
	for _, sched := range s {
		sc := ScheduleConfig{
			Query:    sched.Query.Query,
			Interval: sched.Interval,
			Platform: sched.Platform,
			Snapshot: sched.Snapshot,
		}
		osc.Schedule[sched.VersionedName] = sc
	}

	return c.JSON(http.StatusOK, osc)
}

func (h *Handler) HandleLog(c echo.Context) error {
	var req LogRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusBadRequest, "invalid request", err, nil)
	}

	h.logger.Debug("request", "log", req)

	if err := h.c.IngestOsqueryLogs(c.Request().Context(), models.OsqueryLogBatch{
		NodeKey: req.NodeKey,
		LogType: req.LogType,
		Data:    req.Data,
	}); err != nil {
		// Backpressure: osquery treats 503 as a transient failure and retries
		// on the next interval, which is exactly what the spec asks for. A
		// generic 500 would mark the batch as broken.
		if errors.Is(err, results.ErrBackpressure) {
			return wrapError(http.StatusServiceUnavailable, "results buffer full, retry later", err, nil)
		}
		return wrapError(http.StatusInternalServerError, "error writing logs", err, nil)
	}

	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleDistributedRead(c echo.Context) error {
	var req DistributedReadRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusBadRequest, "invalid request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(http.StatusBadRequest, fmt.Sprintf("invalid request: %s", formatValidationErrors(err)), err, nil)
	}

	queries, err := h.c.ReadDistributedQueries(c.Request().Context(), models.NodeKeyRequest{NodeKey: req.NodeKey})
	if err != nil {
		return wrapError(http.StatusInternalServerError, "error reading distributed queries", err, nil)
	}

	return c.JSON(http.StatusOK, DistributedReadResponse{Queries: queries})
}

func (h *Handler) HandleDistributedWrite(c echo.Context) error {
	var req DistributedWriteRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusBadRequest, "invalid request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(http.StatusBadRequest, fmt.Sprintf("invalid request: %s", formatValidationErrors(err)), err, nil)
	}

	statuses := make(map[string]string, len(req.Statuses))
	for k, v := range req.Statuses {
		statuses[k] = string(v)
	}

	if err := h.c.WriteDistributedQueryResults(c.Request().Context(), models.NodeKeyRequest{NodeKey: req.NodeKey}, req.Queries, statuses, req.Messages); err != nil {
		return wrapError(http.StatusInternalServerError, "error writing distributed query results", err, nil)
	}

	return c.NoContent(http.StatusOK)
}
