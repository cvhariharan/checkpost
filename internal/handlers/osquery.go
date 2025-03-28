package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// CountPerPage used for pagination requests
const CountPerPage = 10

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

	q, err := h.c.CreateQuery(c.Request().Context(), req.Query, req.Description)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "error creating query", err)
	}

	resp := CreateQueryResponse{
		ID: q.UUID,
	}

	return c.JSON(http.StatusCreated, resp)
}

func (h *Handler) HandleQueriesPagination(c echo.Context) error {
	var req PaginateRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusInternalServerError, "invalid request", err)
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
