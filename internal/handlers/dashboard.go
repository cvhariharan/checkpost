package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func (h *Handler) HandleDashboardOverview(c echo.Context) error {
	var req DashboardOverviewRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	overview, err := h.c.DashboardOverview(c.Request().Context(), req.Top)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not get dashboard overview", err, nil)
	}

	c.Response().Header().Set(echo.HeaderCacheControl, "private, max-age=15")
	return c.JSON(http.StatusOK, overview)
}
