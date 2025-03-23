package handlers

import (
	"encoding/json"
	"html/template"
	"io"

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

func NewTemplateRenderer() *Template {
	funcMap := template.FuncMap{
		"toJSON": toJSON,
	}

	t := &Template{
		templates: template.Must(template.New("").Funcs(funcMap).ParseGlob("web/layouts/*.html")),
	}
	template.Must(t.templates.ParseGlob("web/components/*.html"))
	template.Must(t.templates.ParseGlob("web/pages/**/*.html"))
	template.Must(t.templates.ParseGlob("web/pages/*.html"))

	return t
}
