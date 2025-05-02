package handlers

import (
	"fmt"
	"net/http"
	"strings"

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
		return wrapError(http.StatusBadRequest, "invalid enrollment request", err)
	}

	SanitizeStruct(&req)

	if req.EnrollSecret != h.cfg.EnrollmentKey {
		return wrapError(http.StatusUnauthorized, "invalid enrollment key", fmt.Errorf("enrollment key invalid"))
	}

	nodeKey, err := h.c.EnrollNode(c.Request().Context(), req.ToNodeModel())
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not enroll node", err)
	}

	return c.JSON(http.StatusOK, EnrollmentResponse{
		NodeKey:     nodeKey,
		NodeInvalid: false,
	})
}

func (h *Handler) HandleCreateQuery(c echo.Context) error {
	var req CreateQueryRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusInternalServerError, "invalid create query request", err)
	}
	SanitizeStruct(&req)

	if req.Query == "" {
		return wrapError(http.StatusBadRequest, "query cannot be empty", fmt.Errorf("empty query"))
	}

	sqlKeywords := []string{"SELECT", "FROM", "WHERE", "JOIN", "ORDER BY", "GROUP BY", "HAVING", "LIMIT", "INSERT", "UPDATE", "DELETE"}
	validSQL := false

	for _, keyword := range sqlKeywords {
		if strings.Contains(strings.ToUpper(req.Query), keyword) {
			validSQL = true
			break
		}
	}

	if !validSQL {
		return wrapError(http.StatusBadRequest, "invalid SQL query", fmt.Errorf("query does not appear to be valid SQL"))
	}

	q, err := h.c.CreateQuery(c.Request().Context(), req.Title, req.Query, req.Description)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "error creating query", err)
	}

	resp := CreateResponse{
		ID: q.UUID,
	}

	return c.JSON(http.StatusCreated, resp)
}

func (h *Handler) HandleQueriesPagination(c echo.Context) error {
	var req PaginateRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusInternalServerError, "invalid request", err)
	}

	if req.Page < 0 || req.Count < 0 {
		return wrapError(http.StatusInternalServerError, "invalid request, page or count per page cannot be less than 0", fmt.Errorf("page and count per page less than zero"))
	}

	if req.Page > 0 {
		req.Page -= 1
	}

	if req.Count == 0 {
		req.Count = CountPerPage
	}

	queries, totalCount, pageCount, err := h.c.PaginateQueries(c.Request().Context(), req.Page, req.Count)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not get queries", err)
	}

	return c.JSON(http.StatusOK, PaginateQueriesResponse{
		Queries:    queries,
		TotalCount: totalCount,
		PageCount:  pageCount,
	})
}

func (h *Handler) HandleGetQuery(c echo.Context) error {
	var req GetRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusInternalServerError, "invalid request", err)
	}

	if req.ID == "" {
		return wrapError(http.StatusBadRequest, "id cannot be empty", fmt.Errorf("id is empty"))
	}

	q, err := h.c.GetQuery(c.Request().Context(), req.ID)
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error getting query %s", req.ID), err)
	}

	return c.JSON(http.StatusOK, q)
}

func (h *Handler) HandleDeleteQuery(c echo.Context) error {
	var req GetRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusInternalServerError, "invalid request", err)
	}

	if req.ID == "" {
		return wrapError(http.StatusBadRequest, "id cannot be empty", fmt.Errorf("id is empty"))
	}

	if err := h.c.DeleteQuery(c.Request().Context(), req.ID); err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error deleting query %s", req.ID), err)
	}

	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleUpdateQuery(c echo.Context) error {
	var req UpdateQueryRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusInternalServerError, "invalid request", err)
	}

	if req.ID == "" {
		return wrapError(http.StatusBadRequest, "id cannot be empty", fmt.Errorf("id is empty"))
	}

	q, err := h.c.UpdateQuery(c.Request().Context(), req.ID, req.Title, req.Query, req.Description)
	if err != nil {
		return wrapError(http.StatusInternalServerError, fmt.Sprintf("error updating query %s", req.ID), err)
	}

	return c.JSON(http.StatusOK, q)
}

func (h *Handler) HandleOSQueryConfig(c echo.Context) error {
	var req ConfigRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusInternalServerError, "invalid request", err)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(http.StatusBadRequest, fmt.Sprintf("invalid request: %s", formatValidationErrors(err)), err)
	}

	if _, err := h.c.GetNode(c.Request().Context(), req.NodeKey); err != nil {
		return wrapError(http.StatusBadRequest, "error getting node details", err)
	}

	s, _, _, err := h.c.PaginateSchedules(c.Request().Context(), 0, ScheduleMax)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "error getting schedules", err)
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
		osc.Schedule[sched.Title] = sc
	}

	return c.JSON(http.StatusOK, osc)
}

func (h *Handler) HandleLog(c echo.Context) error {
	var req LogRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusBadRequest, "invalid request", err)
	}

	h.logger.Debug("request", "body", req.Data)

	if err := h.c.LogResults(c.Request().Context(), req.LogType, req.Data); err != nil {
		return wrapError(http.StatusInternalServerError, "error writing logs", err)
	}

	return c.NoContent(http.StatusOK)
}
