package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/cvhariharan/checkpost/internal/core"
	"github.com/labstack/echo/v4"
)

func (h *Handler) HandleExportAdHocResults(c echo.Context) error {
	var req AdHocResultsRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	return h.streamExport(c, perHostExportFilename(req.QueryID), func(f *os.File) error {
		return h.c.ExportAdHocQueryResults(c.Request().Context(), req.QueryID, f, exportFormat(c))
	})
}

func (h *Handler) HandleExportQueryRunResults(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	return h.streamExport(c, runExportFilename(req.ID), func(f *os.File) error {
		return h.c.ExportQueryRunResults(c.Request().Context(), req.ID, f, exportFormat(c))
	})
}

func (h *Handler) HandleExportScheduleResults(c echo.Context) error {
	var req GetRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}
	return h.streamExport(c, scheduleExportFilename(req.ID), func(f *os.File) error {
		return h.c.ExportScheduleResults(c.Request().Context(), req.ID, f, exportFormat(c))
	})
}

func (h *Handler) streamExport(c echo.Context, filename string, write func(*os.File) error) error {
	if format := exportFormat(c); format != "csv" {
		return wrapError(http.StatusBadRequest, "unsupported export format", core.ErrExportUnsupported, nil)
	}
	tmp, err := os.CreateTemp("", "ck-http-export-*.csv")
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not create export file", err, nil)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	// write() hands tmp to DuckDB, which COPYs the result set directly into it.
	// Close our write handle and re-open by path so streaming is independent of
	// how DuckDB wrote the file.
	writeErr := write(tmp)
	tmp.Close()
	if writeErr != nil {
		if errors.Is(writeErr, sql.ErrNoRows) {
			return wrapError(http.StatusNotFound, "query results not found", writeErr, nil)
		}
		if errors.Is(writeErr, core.ErrResultNotReady) || errors.Is(writeErr, core.ErrExportUnsupported) || errors.Is(writeErr, core.ErrResultsBackendDisabled) {
			return wrapError(http.StatusConflict, writeErr.Error(), writeErr, nil)
		}
		return wrapError(http.StatusInternalServerError, "could not export query results", writeErr, nil)
	}

	f, err := os.Open(tmpPath)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not read export file", err, nil)
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not stat export file", err, nil)
	}
	header := c.Response().Header()
	header.Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	header.Set(echo.HeaderContentLength, strconv.FormatInt(stat.Size(), 10))
	return c.Stream(http.StatusOK, "text/csv; charset=utf-8", f)
}

func exportFormat(c echo.Context) string {
	if f := c.QueryParam("format"); f != "" {
		return f
	}
	return "csv"
}

func perHostExportFilename(queryID string) string {
	return fmt.Sprintf("query-%s-%s.csv", shortID(queryID), time.Now().Format("20060102"))
}

func runExportFilename(runID string) string {
	return fmt.Sprintf("query-run-%s-all-hosts-%s.csv", shortID(runID), time.Now().Format("20060102"))
}

func scheduleExportFilename(scheduleID string) string {
	return fmt.Sprintf("schedule-%s-%s.csv", shortID(scheduleID), time.Now().Format("20060102"))
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
