package handlers

import (
	"net/http"
	"strings"

	"github.com/cvhariharan/watcher/internal/config"
	"github.com/cvhariharan/watcher/internal/core"
	"github.com/labstack/echo/v4"
)

type Handler struct {
	cfg config.AppConfig
	c   *core.Core
}

func NewHandler(cfg config.AppConfig, c *core.Core) *Handler {
	return &Handler{cfg: cfg, c: c}
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

	c.Logger().Error("request error",
		"status", code,
		"path", c.Request().URL.Path,
		"method", c.Request().Method,
		"error", err.Error(),
		"remote_ip", c.RealIP(),
		"request_id", c.Response().Header().Get("X-Request-ID"))

	if strings.HasPrefix(c.Request().URL.Path, "/view") {
		if err := showErrorPage(c, code, errMsg); err != nil {
			c.Logger().Error(err)
		}
	} else {
		c.JSON(code, map[string]string{
			"error": errMsg,
		})
	}
}
