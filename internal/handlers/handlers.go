package handlers

import (
	"net/http"

	"github.com/cvhariharan/watcher/internal/core"
	"github.com/labstack/echo/v4"
)

type Handler struct {
	c *core.Core
}

func NewHandler(c *core.Core) *Handler {
	return &Handler{c: c}
}

func (h *Handler) HandlePing(c echo.Context) error {
	return c.NoContent(http.StatusOK)
}

func showErrorPage(c echo.Context, errCode int, msg string) error {
	return c.Render(errCode, "base.html", IndexPageData{
		Title:        "Machines",
		Active:       "error",
		ErrorCode:    errCode,
		ErrorMessage: msg,
	})
}

func ErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	code := http.StatusInternalServerError
	errMsg := "error processing the request"
	if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
		errMsg = he.Message.(string)
	}

	c.Logger().Error(err)

	if err := showErrorPage(c, code, errMsg); err != nil {
		c.Logger().Error(err)
	}
}
