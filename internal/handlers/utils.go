package handlers

import (
	"encoding/json"
	"html/template"
	"io"
	"reflect"
	"runtime"
	"strings"

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

// SanitizeString removes null bytes and invalid UTF-8 characters
func SanitizeString(s string) string {
	return strings.ReplaceAll(s, "\x00", "")
}

// SanitizeStruct recursively sanitizes all string fields in a struct
func SanitizeStruct(obj interface{}) {
	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return
	}

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)

		if field.Kind() == reflect.String && field.CanSet() {
			field.SetString(SanitizeString(field.String()))
		} else if field.Kind() == reflect.Struct {
			SanitizeStruct(field.Addr().Interface())
		} else if field.Kind() == reflect.Ptr && !field.IsNil() && field.Elem().Kind() == reflect.Struct {
			SanitizeStruct(field.Interface())
		}
	}
}

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
