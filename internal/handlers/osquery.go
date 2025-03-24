package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func (h *Handler) HandleEnrollment(c echo.Context) error {
	var req EnrollmentRequest
	if err := c.Bind(&req); err != nil {
		c.Logger().Errorf("error binding enrollment request: %w", err)
		return echo.NewHTTPError(http.StatusBadRequest, "invalid enrollment request")
	}

	if req.EnrollSecret != h.cfg.EnrollmentKey {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid enrollment key")
	}

	nodeKey, err := h.c.EnrollNode(c.Request().Context(), req.ToNodeModel())
	if err != nil {
		c.Logger().Errorf("could not enroll node: %w", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "could not enroll node")
	}

	return c.JSON(http.StatusOK, EnrollmentResponse{
		NodeKey:     nodeKey,
		NodeInvalid: false,
	})
}
