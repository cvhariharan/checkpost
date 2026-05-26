package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func (h *Handler) HandleMetricSchemas(c echo.Context) error {
	return c.JSON(http.StatusOK, h.c.GetMetricSchemas())
}
