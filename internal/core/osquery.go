package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/cvhariharan/checkpost/internal/repo"
	"github.com/cvhariharan/checkpost/internal/results"
	"github.com/google/uuid"
)

const MaxLogCount = 1000

func (c *Core) ReadDistributedQueries(ctx context.Context, req models.NodeKeyRequest) (map[string]string, error) {
	node, err := c.GetNode(ctx, req)
	if err != nil {
		return nil, err
	}
	if err := c.RecordNodeHeartbeat(ctx, req); err != nil {
		return nil, err
	}

	policyDue := c.nodePolicyDue(node)
	var policies []repo.Policy
	if policyDue {
		policies, err = c.store.ListEnabledPoliciesForNode(ctx, repo.ListEnabledPoliciesForNodeParams{
			NodeID:       node.ID,
			NodePlatform: strings.ToLower(strings.TrimSpace(node.Platform)),
		})
		if err != nil {
			return nil, fmt.Errorf("list enabled policies for node: %w", err)
		}
	}

	pending, err := c.pendingMachineQueries(ctx, node)
	if err != nil {
		return nil, err
	}
	yaraQueries, err := c.PendingYaraQueries(ctx, node)
	if err != nil {
		return nil, err
	}

	queries := make(map[string]string, len(pending)+len(yaraQueries))
	for _, item := range pending {
		queries[item.ID] = item.Query
	}
	for id, query := range yaraQueries {
		queries[id] = query
	}

	if policyDue {
		if len(policies) == 0 {
			queries[noPoliciesQueryID] = "SELECT 1"
		} else {
			for _, policy := range policies {
				queries[fmt.Sprintf(policyQueryFormat, policy.Uuid.String())] = policy.Query
			}
		}
	}

	return queries, nil
}

func (c *Core) WriteDistributedQueryResults(ctx context.Context, req models.NodeKeyRequest, results map[string]interface{}, statuses map[string]string, messages map[string]string) error {
	node, err := c.GetNode(ctx, req)
	if err != nil {
		return err
	}
	if err := c.RecordNodeHeartbeat(ctx, req); err != nil {
		return err
	}

	yaraErr := c.deliverYaraResults(ctx, node, results, statuses, messages)
	adHocErr := c.deliverAdHocResults(ctx, node, results, statuses, messages)
	policyErr := c.recordPolicyResults(ctx, node, results, statuses, messages)

	if yaraErr != nil {
		return yaraErr
	}
	if adHocErr != nil {
		return adHocErr
	}
	return policyErr
}

func (c *Core) deliverYaraResults(ctx context.Context, node models.Node, results map[string]interface{}, statuses, messages map[string]string) error {
	var firstErr error
	seen := make(map[string]struct{}, len(results)+len(statuses))
	for queryID, rows := range results {
		seen[queryID] = struct{}{}
		handled, err := c.completeYaraResult(ctx, node, queryID, rows, distributedErrorMessage(statuses[queryID], messages[queryID]))
		if handled && err != nil {
			c.logger.Error("record yara query result", "node_id", node.ID, "query_id", queryID, "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	for queryID, status := range statuses {
		if _, ok := seen[queryID]; ok || !strings.HasPrefix(queryID, yaraQueryPrefix) {
			continue
		}
		handled, err := c.completeYaraResult(ctx, node, queryID, nil, distributedErrorMessage(status, messages[queryID]))
		if handled && err != nil {
			c.logger.Error("record yara query status", "node_id", node.ID, "query_id", queryID, "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (c *Core) deliverAdHocResults(ctx context.Context, node models.Node, results map[string]interface{}, statuses, messages map[string]string) error {
	var firstErr error
	for queryID, rows := range results {
		if isPolicyDistributedQuery(queryID) {
			continue
		}
		if strings.HasPrefix(queryID, yaraQueryPrefix) {
			continue
		}
		_, _, err := c.completeAdHocResult(ctx, queryID, rows, distributedErrorMessage(statuses[queryID], messages[queryID]))
		if err != nil {
			c.logger.Error("record ad-hoc query result", "node_id", node.ID, "query_id", queryID, "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func distributedErrorMessage(status, message string) string {
	if status == "" || status == "0" {
		return ""
	}
	if msg := strings.TrimSpace(message); msg != "" {
		return msg
	}
	return fmt.Sprintf("osquery returned status %s", status)
}

func (c *Core) IngestOsqueryLogs(ctx context.Context, batch models.OsqueryLogBatch) error {
	if batch.LogType != "result" && batch.LogType != "status" {
		return ErrInvalidLogType
	}

	nodeID, err := uuid.Parse(batch.NodeKey)
	if err != nil {
		return fmt.Errorf("parse node key: %w", err)
	}

	node, err := c.store.GetNodeByKey(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("get node for log ingestion: %w", err)
	}

	logs := batch.Data
	if len(logs) > MaxLogCount {
		logs = logs[:MaxLogCount]
		c.logger.Warn("log data truncated due to limit", "original_count", len(batch.Data), "max_count", MaxLogCount)
	}

	switch batch.LogType {
	case "result":
		return c.ingestResultLogs(ctx, node.ID, c.parseResultLogs(logs))
	case "status":
		return c.ingestStatusLogs(ctx, node.ID, parseStatusLogs(logs))
	default:
		return ErrInvalidLogType
	}
}

// resultLog is the parsed shape of one osquery result-log entry. The wire
// format is a JSON object whose schema depends on `action`: differential
// logs ("added"/"removed") carry a single `columns` map; snapshot logs
// carry a `snapshot` array of row maps.
type resultLog struct {
	versionedName string // "schedule_name/v3"
	scheduleName  string // "schedule_name"
	sqlVersion    int32  // 3
	action        string // "added" | "removed" | "snapshot"
	calendarTime  string
	unixTime      time.Time
	columns       map[string]interface{}   // differential row payload
	snapshot      []map[string]interface{} // snapshot row payload
}

func (c *Core) parseResultLogs(logs []map[string]interface{}) []resultLog {
	parsed := make([]resultLog, 0, len(logs))
	for _, raw := range logs {
		log, ok := parseResultLog(raw)
		if !ok {
			c.logger.Debug("skip result with unparseable schedule name", "name", stringValue(raw["name"]))
			continue
		}
		parsed = append(parsed, log)
	}
	return parsed
}

func (c *Core) ingestResultLogs(ctx context.Context, nodeID int64, logs []resultLog) error {
	if c.sink == nil {
		return errors.New("results sink not configured")
	}
	if len(logs) == 0 {
		return nil
	}

	// One batched lookup for every unique schedule referenced in the batch.
	nameSet := make(map[string]struct{})
	for _, log := range logs {
		nameSet[log.scheduleName] = struct{}{}
	}
	names := make([]string, 0, len(nameSet))
	for n := range nameSet {
		names = append(names, n)
	}
	schedules, err := c.store.ListSchedulesByNames(ctx, names)
	if err != nil {
		return fmt.Errorf("lookup schedules: %w", err)
	}
	schedulesByName := make(map[string]repo.Schedule, len(schedules))
	for _, sched := range schedules {
		schedulesByName[sched.Name] = sched
	}

	// Submit rows under the matched schedule UUID + sql_version. Logs whose
	// schedule isn't in the DB are silently skipped (agent saw the schedule
	// before we deleted it, or never enrolled it).
	for _, log := range logs {
		sched, ok := schedulesByName[log.scheduleName]
		if !ok {
			continue
		}
		rows := buildResultRows(log, nodeID)
		if len(rows) == 0 {
			continue
		}
		if err := c.sink.Submit(ctx, results.Batch{
			ScheduleUUID: sched.Uuid,
			SQLVersion:   log.sqlVersion,
			ScheduleName: sched.Name,
			Snapshot:     sched.Snapshot,
			Rows:         rows,
		}); err != nil {
			return fmt.Errorf("submit results for %s: %w", log.versionedName, err)
		}
		if sched.IsSystem && sched.Snapshot {
			c.applySystemMetric(ctx, nodeID, sched.Name, rows, log.unixTime)
		}
	}
	return nil
}

// applySystemMetric reduces a system schedule's result batch via the
// registry and upserts the snapshot into node_metrics. Failures are logged
// but never bubbled — the parquet write is authoritative.
func (c *Core) applySystemMetric(ctx context.Context, nodeID int64, scheduleName string, rows []results.Row, collectedAt time.Time) {
	snap, ok := c.systemMetrics.Apply(scheduleName, rows)
	if !ok {
		return
	}
	value, err := json.Marshal(snap.Value)
	if err != nil {
		c.logger.Warn("marshal system metric", "node_id", nodeID, "kind", snap.Kind, "error", err)
		return
	}
	if err := c.store.UpsertNodeMetric(ctx, repo.UpsertNodeMetricParams{
		NodeID:      nodeID,
		Kind:        string(snap.Kind),
		Value:       value,
		CollectedAt: collectedAt,
	}); err != nil {
		c.logger.Warn("upsert node metric", "node_id", nodeID, "kind", snap.Kind, "error", err)
	}
}

func parseResultLog(raw map[string]interface{}) (resultLog, bool) {
	versionedName := strings.TrimSpace(stringValue(raw["name"]))
	if versionedName == "" {
		return resultLog{}, false
	}
	scheduleName, sqlVersion, ok := parseVersionedName(versionedName)
	if !ok {
		return resultLog{}, false
	}

	action := strings.TrimSpace(stringValue(raw["action"]))
	if action == "" {
		action = "snapshot"
	}

	log := resultLog{
		versionedName: versionedName,
		scheduleName:  scheduleName,
		sqlVersion:    sqlVersion,
		action:        action,
		calendarTime:  stringValue(raw["calendarTime"]),
		unixTime:      ingestUnixTime(raw),
	}
	if cols, ok := raw["columns"].(map[string]interface{}); ok {
		log.columns = cols
	}
	if snap, ok := raw["snapshot"].([]interface{}); ok {
		log.snapshot = make([]map[string]interface{}, 0, len(snap))
		for _, v := range snap {
			if rowMap, ok := v.(map[string]interface{}); ok {
				log.snapshot = append(log.snapshot, rowMap)
			}
		}
	}
	return log, true
}

func parseVersionedName(versioned string) (string, int32, bool) {
	idx := strings.LastIndex(versioned, "/v")
	if idx <= 0 || idx == len(versioned)-2 {
		return "", 0, false
	}
	v, err := strconv.ParseInt(versioned[idx+2:], 10, 32)
	if err != nil || v <= 0 {
		return "", 0, false
	}
	return versioned[:idx], int32(v), true
}

func buildResultRows(log resultLog, nodeID int64) []results.Row {
	// Differential: one row per log entry from `columns`.
	if log.action != "snapshot" {
		if log.columns == nil {
			return nil
		}
		return []results.Row{resultRowFromMap(log.columns, log.action, nodeID, log.unixTime, log.calendarTime)}
	}

	// Snapshot: many rows from `snapshot`. Osquery sometimes emits
	// consecutive duplicates; collapse them by row hash.
	rows := make([]results.Row, 0, len(log.snapshot))
	var lastHash []byte
	for _, rowMap := range log.snapshot {
		row := resultRowFromMap(rowMap, log.action, nodeID, log.unixTime, log.calendarTime)
		if bytes.Equal(row.RowHash, lastHash) {
			continue
		}
		lastHash = row.RowHash
		rows = append(rows, row)
	}
	return rows
}

func resultRowFromMap(rowMap map[string]interface{}, action string, nodeID int64, ut time.Time, calendarTime string) results.Row {
	columns := make(map[string]string, len(rowMap))
	for k, v := range rowMap {
		if strings.TrimSpace(k) != "" {
			columns[k] = stringValue(v)
		}
	}
	return results.Row{
		NodeID:       nodeID,
		UnixTime:     ut,
		CalendarTime: calendarTime,
		Action:       action,
		RowHash:      rowHash(columns),
		Columns:      columns,
	}
}

func rowHash(columns map[string]string) []byte {
	keys := make([]string, 0, len(columns))
	for k := range columns {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
		h.Write([]byte(columns[k]))
		h.Write([]byte{0})
	}
	return h.Sum(nil)
}

func ingestUnixTime(raw map[string]interface{}) time.Time {
	if hasValue(raw["unixTime"]) {
		return unixTime(raw["unixTime"]).UTC()
	}
	return time.Now().UTC()
}

// statusLog is the parsed shape of one osquery status-log entry. Severity
// follows osquery's INFO=0/WARNING=1/ERROR=2 convention.
type statusLog struct {
	calendarTime string
	fileName     string
	line         int32
	message      string
	severity     int32
	unixTime     sql.NullTime
	version      string
}

func parseStatusLogs(logs []map[string]interface{}) []statusLog {
	out := make([]statusLog, 0, len(logs))
	for _, raw := range logs {
		out = append(out, statusLog{
			calendarTime: stringValue(raw["calendarTime"]),
			fileName:     stringValue(firstValue(raw, "filename", "file_name")),
			line:         int32(int64Value(raw["line"])),
			message:      stringValue(raw["message"]),
			severity:     int32(int64Value(raw["severity"])),
			unixTime:     sql.NullTime{Time: unixTime(raw["unixTime"]), Valid: hasValue(raw["unixTime"])},
			version:      stringValue(raw["version"]),
		})
	}
	return out
}

func (c *Core) ingestStatusLogs(ctx context.Context, nodeID int64, logs []statusLog) error {
	params := repo.InsertStatusLogsTxParams{Logs: make([]repo.CreateStatusLogParams, 0, len(logs))}
	for _, log := range logs {
		params.Logs = append(params.Logs, repo.CreateStatusLogParams{
			NodeID:       nodeID,
			CalendarTime: log.calendarTime,
			FileName:     log.fileName,
			Line:         log.line,
			Message:      log.message,
			Severity:     log.severity,
			UnixTime:     log.unixTime,
			Version:      log.version,
		})
	}
	return c.store.InsertStatusLogsTx(ctx, params)
}

func unixTime(value interface{}) time.Time {
	return time.Unix(int64Value(value), 0)
}

func hasValue(value interface{}) bool {
	return value != nil
}

func firstValue(m map[string]interface{}, keys ...string) interface{} {
	for _, key := range keys {
		if value, ok := m[key]; ok {
			return value
		}
	}
	return nil
}

func stringValue(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case json.Number:
		return v.String()
	case float64:
		if math.Trunc(v) == v {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprint(v)
	}
}

func int64Value(value interface{}) int64 {
	switch v := value.(type) {
	case nil:
		return 0
	case int64:
		return v
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case uint64:
		return int64(v)
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	case json.Number:
		i, _ := v.Int64()
		return i
	case string:
		i, _ := strconv.ParseInt(v, 10, 64)
		return i
	default:
		return 0
	}
}
