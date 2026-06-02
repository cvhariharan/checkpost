package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/cvhariharan/checkpost/internal/results"
	"github.com/labstack/echo/v4"
)

const (
	// CountPerPage used for pagination requests
	CountPerPage = 10

	// MaxResultsPerPage caps how many schedule-result rows can be requested
	// in one page; protects DuckDB and the response allocator from OOM.
	MaxResultsPerPage = 1000

	// Maximum number of schedules that will be returned in osquery config
	ScheduleMax = 100
)

func (h *Handler) HandleEnrollment(c echo.Context) error {
	var req EnrollmentRequest
	if err := h.bindAndValidate(c, &req, EnrollmentResponse{NodeInvalid: true}); err != nil {
		return err
	}

	if req.EnrollSecret != h.cfg.EnrollmentKey {
		return wrapError(http.StatusUnauthorized, "invalid enrollment key", fmt.Errorf("enrollment key invalid"), EnrollmentResponse{NodeInvalid: true})
	}

	creds, err := h.c.EnrollNode(c.Request().Context(), req.ToNodeModel())
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not enroll node", err, EnrollmentResponse{NodeInvalid: true})
	}

	return c.JSON(http.StatusOK, EnrollmentResponse{
		NodeKey:     creds.NodeKey,
		NodeInvalid: false,
	})
}

func (h *Handler) HandleOSQueryConfig(c echo.Context) error {
	var req ConfigRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	s, err := h.c.ListEnabledSchedulesForNode(c.Request().Context(), models.NodeKeyRequest{NodeKey: req.NodeKey}, ScheduleMax)
	if err != nil {
		return wrapError(http.StatusBadRequest, "error getting schedules for node", err, EnrollmentResponse{NodeInvalid: true})
	}
	yaraURLs, err := h.c.YaraSignatureURLAllowlist(c.Request().Context(), models.NodeKeyRequest{NodeKey: req.NodeKey})
	if err != nil {
		return wrapError(http.StatusBadRequest, "error getting YARA allowlist for node", err, EnrollmentResponse{NodeInvalid: true})
	}

	h.logger.Debug("config", "response", s)

	osc := OSQueryConfigResponse{
		Schedule: make(map[string]ScheduleConfig),
		Yara:     YaraConfig{SignatureURLs: yaraURLs},
	}
	for _, sched := range s {
		sc := ScheduleConfig{
			Query:    sched.SQL,
			Interval: sched.Interval,
			Platform: sched.Platform,
			Snapshot: sched.Snapshot,
		}
		osc.Schedule[sched.VersionedName] = sc
	}

	return c.JSON(http.StatusOK, osc)
}

func (h *Handler) HandleLog(c echo.Context) error {
	var req LogRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	h.logger.Debug("request", "log", req)

	if err := h.c.IngestOsqueryLogs(c.Request().Context(), models.OsqueryLogBatch{
		NodeKey: req.NodeKey,
		LogType: req.LogType,
		Data:    req.Data,
	}); err != nil {
		// Backpressure: osquery treats 503 as a transient failure and retries
		// on the next interval, which is exactly what the spec asks for. A
		// generic 500 would mark the batch as broken.
		if errors.Is(err, results.ErrBackpressure) {
			return wrapError(http.StatusServiceUnavailable, "results buffer full, retry later", err, nil)
		}
		return wrapError(http.StatusInternalServerError, "error writing logs", err, nil)
	}

	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleDistributedRead(c echo.Context) error {
	var req DistributedReadRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	queries, err := h.c.ReadDistributedQueries(c.Request().Context(), models.NodeKeyRequest{NodeKey: req.NodeKey})
	if err != nil {
		return wrapError(http.StatusInternalServerError, "error reading distributed queries", err, nil)
	}

	return c.JSON(http.StatusOK, DistributedReadResponse{Queries: queries})
}

func (h *Handler) HandleDistributedWrite(c echo.Context) error {
	var req DistributedWriteRequest
	if err := h.bindAndValidate(c, &req, nil); err != nil {
		return err
	}

	statuses := make(map[string]string, len(req.Statuses))
	for k, v := range req.Statuses {
		statuses[k] = string(v)
	}

	if err := h.c.WriteDistributedQueryResults(c.Request().Context(), models.NodeKeyRequest{NodeKey: req.NodeKey}, req.Queries, statuses, req.Messages); err != nil {
		return wrapError(http.StatusInternalServerError, "error writing distributed query results", err, nil)
	}

	return c.NoContent(http.StatusOK)
}
