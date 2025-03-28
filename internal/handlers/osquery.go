package handlers

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

func (h *Handler) HandleEnrollment(c echo.Context) error {
	var req EnrollmentRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusBadRequest, "invalid enrollment request", err)
	}

	SanitizeStruct(&req)

	c.Logger().Debugf("%+v", req)

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

	q, err := h.c.CreateQuery(c.Request().Context(), req.Query, req.Description)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "error creating query", err)
	}

	resp := CreateQueryResponse{
		ID: q.UUID,
	}

	return c.JSON(http.StatusCreated, resp)
}
