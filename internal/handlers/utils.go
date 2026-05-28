package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"runtime"

	"github.com/labstack/echo/v4"
)

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data any, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

// Convert any value to a JSON string
func toJSON(v any) string {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(jsonBytes)
}

// func NewTemplateRenderer() *Template {
// 	funcMap := template.FuncMap{
// 		"toJSON": toJSON,
// 	}

// 	t := &Template{
// 		templates: template.Must(template.New("").Funcs(funcMap).ParseGlob("web/layouts/*.html")),
// 	}
// 	template.Must(t.templates.ParseGlob("web/components/*.html"))
// 	template.Must(t.templates.ParseGlob("web/pages/**/*.html"))
// 	template.Must(t.templates.ParseGlob("web/pages/*.html"))

// 	return t
// }

type HTTPError struct {
	code           int
	msg            string
	err            error
	file           string
	line           int
	customResponse interface{}
}

func (h *HTTPError) Error() string {
	return h.err.Error()
}

func wrapError(code int, msg string, err error, customResponse interface{}) error {
	he := &HTTPError{
		code:           code,
		msg:            msg,
		err:            err,
		file:           "unknown",
		line:           -1,
		customResponse: customResponse,
	}
	_, f, l, ok := runtime.Caller(1)
	if ok {
		he.file = f
		he.line = l
	}

	return he
}

func (h *Handler) bindAndValidate(c echo.Context, req interface{}, customResponse interface{}) error {
	if err := c.Bind(req); err != nil {
		return wrapError(http.StatusBadRequest, "invalid request", err, customResponse)
	}
	if err := h.validate.Struct(req); err != nil {
		return wrapError(http.StatusBadRequest, fmt.Sprintf("invalid request: %s", formatValidationErrors(err)), err, customResponse)
	}
	return nil
}
