package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func (h *Handler) HandlePacksList(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"packs": []Pack{},
	})
}
