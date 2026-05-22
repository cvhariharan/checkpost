package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/cvhariharan/watcher/internal/models"
	"github.com/cvhariharan/watcher/internal/repo"
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
		policies, err = c.store.ListEnabledPoliciesForPlatform(ctx, strings.ToLower(strings.TrimSpace(node.Platform)))
		if err != nil {
			return nil, fmt.Errorf("list enabled policies for node: %w", err)
		}
	}

	pending, err := c.pendingMachineQueries(ctx, node)
	if err != nil {
		return nil, err
	}

	queries := make(map[string]string, len(pending))
	for _, item := range pending {
		queries[item.ID] = item.Query
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

	adHocErr := c.deliverAdHocResults(ctx, node, results, statuses, messages)
	policyErr := c.recordPolicyResults(ctx, node, results, statuses, messages)

	if adHocErr != nil {
		return adHocErr
	}
	return policyErr
}

func (c *Core) deliverAdHocResults(ctx context.Context, node models.Node, results map[string]interface{}, statuses, messages map[string]string) error {
	var firstErr error
	for queryID, rows := range results {
		if isPolicyDistributedQuery(queryID) {
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
		return c.ingestResultLogs(ctx, node.ID, logs)
	case "status":
		return c.ingestStatusLogs(ctx, node.ID, logs)
	default:
		return ErrInvalidLogType
	}
}

func (c *Core) ingestResultLogs(ctx context.Context, nodeID int64, logs []map[string]interface{}) error {
	for _, raw := range logs {
		action := stringValue(raw["action"])
		rows := resultRows(raw, action)
		name := stringValue(raw["name"])

		params := repo.InsertResultBatchTxParams{
			Batch: repo.CreateResultBatchParams{
				NodeID:       nodeID,
				ScheduleName: name,
				Action:       defaultString(action, "snapshot"),
				CalendarTime: stringValue(raw["calendarTime"]),
				Counter:      int64Value(raw["counter"]),
				Epoch:        int64Value(raw["epoch"]),
				Numerics:     boolValue(raw["numerics"]),
				UnixTime:     sql.NullTime{Time: unixTime(raw["unixTime"]), Valid: hasValue(raw["unixTime"])},
				IsSystem:     c.systemSchedulesMap[name],
			},
			Rows: rows,
		}
		if err := c.store.InsertResultBatchTx(ctx, params); err != nil {
			return fmt.Errorf("insert result batch: %w", err)
		}
	}
	return nil
}

func (c *Core) ingestStatusLogs(ctx context.Context, nodeID int64, logs []map[string]interface{}) error {
	params := repo.InsertStatusLogsTxParams{Logs: make([]repo.CreateStatusLogParams, 0, len(logs))}
	for _, raw := range logs {
		params.Logs = append(params.Logs, repo.CreateStatusLogParams{
			NodeID:       nodeID,
			CalendarTime: stringValue(raw["calendarTime"]),
			FileName:     stringValue(firstValue(raw, "filename", "file_name")),
			Line:         int32(int64Value(raw["line"])),
			Message:      stringValue(raw["message"]),
			Severity:     int32(int64Value(raw["severity"])),
			UnixTime:     sql.NullTime{Time: unixTime(raw["unixTime"]), Valid: hasValue(raw["unixTime"])},
			Version:      stringValue(raw["version"]),
		})
	}
	return c.store.InsertStatusLogsTx(ctx, params)
}

func resultRows(raw map[string]interface{}, action string) []repo.InsertResultRowTxParams {
	key := "columns"
	if action == "snapshot" {
		key = "snapshot"
	}

	values, ok := raw[key].([]interface{})
	if !ok {
		return nil
	}

	rows := make([]repo.InsertResultRowTxParams, 0, len(values))
	for i, value := range values {
		rowMap, ok := value.(map[string]interface{})
		if !ok {
			continue
		}

		cells := make([]repo.CreateResultCellTxParams, 0, len(rowMap))
		for k, v := range rowMap {
			if strings.TrimSpace(k) == "" {
				continue
			}
			cells = append(cells, repo.CreateResultCellTxParams{
				ColumnName: k,
				ValueText:  stringValue(v),
			})
		}

		rows = append(rows, repo.InsertResultRowTxParams{
			RowIndex: int32(i),
			Cells:    cells,
		})
	}
	return rows
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

func boolValue(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		b, _ := strconv.ParseBool(v)
		return b
	default:
		return false
	}
}
