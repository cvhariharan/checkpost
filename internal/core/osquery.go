package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cvhariharan/watcher/internal/models"
	"github.com/cvhariharan/watcher/internal/repo"
	"github.com/google/uuid"
)

const MaxLogCount = 1000

var (
	ErrInvalidLogType = errors.New("invalid log type")
	ErrInvalidQuery   = errors.New("invalid query")
)

type Core struct {
	store  repo.Store
	logger *slog.Logger

	systemSchedulesMap map[string]bool

	adHocMu           sync.Mutex
	adHocPending      map[string][]models.MachineQueryResult
	adHocHistory      map[string][]models.MachineQueryResult
	adHocNodeKeyToID  map[string]string
	adHocWaiters      map[string]chan models.MachineQueryResult
	adHocQueryTimeout time.Duration
}

func NewCore(logger *slog.Logger, store repo.Store) (*Core, error) {
	schedules, err := store.ListSystemScheduleNames(context.Background())
	if err != nil {
		return nil, fmt.Errorf("load system schedules: %w", err)
	}

	m := make(map[string]bool, len(schedules))
	for _, sched := range schedules {
		m[sched] = true
	}

	return &Core{
		store:              store,
		logger:             logger.WithGroup("core"),
		systemSchedulesMap: m,
		adHocPending:       make(map[string][]models.MachineQueryResult),
		adHocHistory:       make(map[string][]models.MachineQueryResult),
		adHocNodeKeyToID:   make(map[string]string),
		adHocWaiters:       make(map[string]chan models.MachineQueryResult),
		adHocQueryTimeout:  20 * time.Second,
	}, nil
}

func (c *Core) EnrollNode(ctx context.Context, node models.NodeEnrollment) (models.NodeCredentials, error) {
	created, err := c.store.CreateNode(ctx, repo.CreateNodeParams{
		HostIdentifier: node.HostIdentifier,
		Hostname:       firstNonEmpty(node.HostDetails.System.Hostname, node.HostDetails.System.ComputerName, node.HostDetails.System.LocalHostname, node.HostIdentifier),
		Platform:       firstNonEmpty(node.HostDetails.OSVersion.Platform, node.HostDetails.Platform.Vendor),
		OsName:         node.HostDetails.OSVersion.Name,
		OsVersion:      node.HostDetails.OSVersion.Version,
		OsqueryVersion: node.HostDetails.OSQuery.Version,
		HardwareSerial: node.HostDetails.System.HardwareSerial,
	})
	if err != nil {
		return models.NodeCredentials{}, fmt.Errorf("create node: %w", err)
	}

	return models.NodeCredentials{NodeKey: created.NodeKey.String()}, nil
}

func (c *Core) GetNode(ctx context.Context, req models.NodeKeyRequest) (models.Node, error) {
	id, err := uuid.Parse(req.NodeKey)
	if err != nil {
		return models.Node{}, fmt.Errorf("parse node key: %w", err)
	}

	node, err := c.store.GetNodeByKey(ctx, id)
	if err != nil {
		return models.Node{}, fmt.Errorf("get node: %w", err)
	}

	return toModelNode(node), nil
}

func (c *Core) GetNodeByID(ctx context.Context, req models.NodeIdentity) (models.Node, error) {
	if strings.TrimSpace(req.ID) == "" {
		return models.Node{}, fmt.Errorf("node id cannot be empty")
	}

	node, err := c.store.GetNodeByUUID(ctx, req.ID)
	if err != nil {
		return models.Node{}, fmt.Errorf("get node: %w", err)
	}

	return toModelNode(node), nil
}

func (c *Core) PaginateNodes(ctx context.Context, req models.PageRequest) (models.Page[models.Node], error) {
	page := req.Page
	countPerPage := req.Count
	if countPerPage <= 0 {
		countPerPage = 10
	}
	if page < 0 {
		page = 0
	}

	rows, err := c.store.ListNodes(ctx, repo.ListNodesParams{
		Limit:  int32(countPerPage),
		Offset: int32(page * countPerPage),
	})
	if err != nil {
		return models.Page[models.Node]{}, fmt.Errorf("list nodes: %w", err)
	}

	out := make([]models.Node, 0, len(rows))
	totalCount := 0
	for _, row := range rows {
		out = append(out, toModelNodeRow(row))
		totalCount = int(row.TotalCount)
	}

	return models.Page[models.Node]{
		Items:      out,
		TotalCount: totalCount,
		PageCount:  pageCountFor(totalCount, countPerPage),
	}, nil
}

func (c *Core) ListMachineQueries(ctx context.Context, req models.NodeIdentity) ([]models.MachineQueryResult, error) {
	node, err := c.GetNodeByID(ctx, req)
	if err != nil {
		return nil, err
	}

	c.adHocMu.Lock()
	defer c.adHocMu.Unlock()

	history := c.adHocHistory[node.UUID]
	out := make([]models.MachineQueryResult, len(history))
	copy(out, history)
	return out, nil
}

func (c *Core) ExecuteMachineQuery(ctx context.Context, req models.MachineQueryRequest) (models.MachineQueryResult, error) {
	if err := validateQuery(req.Query); err != nil {
		return models.MachineQueryResult{}, err
	}

	node, err := c.GetNodeByID(ctx, models.NodeIdentity{ID: req.NodeUUID})
	if err != nil {
		return models.MachineQueryResult{}, err
	}

	result := models.MachineQueryResult{
		ID:        uuid.NewString(),
		Query:     req.Query,
		Status:    "pending",
		Timestamp: time.Now().UTC(),
	}
	waiter := make(chan models.MachineQueryResult, 1)

	c.adHocMu.Lock()
	c.adHocPending[node.NodeKey] = append(c.adHocPending[node.NodeKey], result)
	c.adHocHistory[node.UUID] = append([]models.MachineQueryResult{result}, c.adHocHistory[node.UUID]...)
	c.adHocNodeKeyToID[node.NodeKey] = node.UUID
	c.adHocWaiters[result.ID] = waiter
	c.adHocMu.Unlock()

	select {
	case completed := <-waiter:
		return completed, nil
	case <-ctx.Done():
		return result, ctx.Err()
	case <-time.After(c.adHocQueryTimeout):
		return result, nil
	}
}

func (c *Core) ReadDistributedQueries(ctx context.Context, req models.NodeKeyRequest) (map[string]string, error) {
	node, err := c.GetNode(ctx, req)
	if err != nil {
		return nil, err
	}

	c.adHocMu.Lock()
	defer c.adHocMu.Unlock()

	pending := c.adHocPending[node.NodeKey]
	delete(c.adHocPending, node.NodeKey)
	c.adHocNodeKeyToID[node.NodeKey] = node.UUID

	queries := make(map[string]string, len(pending))
	for _, item := range pending {
		queries[item.ID] = item.Query
	}
	return queries, nil
}

func (c *Core) WriteDistributedQueryResults(ctx context.Context, req models.NodeKeyRequest, results map[string]interface{}) error {
	node, err := c.GetNode(ctx, req)
	if err != nil {
		return err
	}

	c.adHocMu.Lock()
	defer c.adHocMu.Unlock()

	c.adHocNodeKeyToID[node.NodeKey] = node.UUID
	for queryID, rows := range results {
		completed := c.updateAdHocResultLocked(node.UUID, queryID, rows, "")
		if waiter, ok := c.adHocWaiters[queryID]; ok {
			waiter <- completed
			close(waiter)
			delete(c.adHocWaiters, queryID)
		}
	}

	return nil
}

func (c *Core) updateAdHocResultLocked(nodeUUID, queryID string, rows interface{}, errMsg string) models.MachineQueryResult {
	status := "complete"
	if errMsg != "" {
		status = "error"
	}

	history := c.adHocHistory[nodeUUID]
	for i, item := range history {
		if item.ID != queryID {
			continue
		}

		history[i].Status = status
		history[i].Results = rows
		history[i].Error = errMsg
		history[i].Timestamp = time.Now().UTC()
		c.adHocHistory[nodeUUID] = history
		return history[i]
	}

	completed := models.MachineQueryResult{
		ID:        queryID,
		Status:    status,
		Timestamp: time.Now().UTC(),
		Results:   rows,
		Error:     errMsg,
	}
	c.adHocHistory[nodeUUID] = append([]models.MachineQueryResult{completed}, history...)
	return completed
}

func (c *Core) CreateQuery(ctx context.Context, req models.CreateQuery) (models.Query, error) {
	if err := validateQuery(req.SQL); err != nil {
		return models.Query{}, err
	}

	q, err := c.store.CreateQuery(ctx, repo.CreateQueryParams{
		Name:        req.Name,
		Sql:         req.SQL,
		Description: req.Description,
		IsSystem:    req.IsSystem,
	})
	if err != nil {
		return models.Query{}, fmt.Errorf("create query: %w", err)
	}

	return toModelQuery(q), nil
}

func (c *Core) GetQuery(ctx context.Context, req models.ResourceID) (models.Query, error) {
	queryID, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.Query{}, fmt.Errorf("parse query uuid: %w", err)
	}

	q, err := c.store.GetQueryByUUID(ctx, queryID)
	if err != nil {
		return models.Query{}, fmt.Errorf("get query: %w", err)
	}

	return toModelQuery(q), nil
}

func (c *Core) PaginateQueries(ctx context.Context, req models.PageRequest) (models.Page[models.Query], error) {
	page := req.Page
	countPerPage := req.Count
	if countPerPage <= 0 {
		countPerPage = 10
	}
	if page < 0 {
		page = 0
	}

	rows, err := c.store.ListQueries(ctx, repo.ListQueriesParams{
		Limit:  int32(countPerPage),
		Offset: int32(page * countPerPage),
	})
	if err != nil {
		return models.Page[models.Query]{}, fmt.Errorf("list queries: %w", err)
	}

	out := make([]models.Query, 0, len(rows))
	totalCount := 0
	for _, row := range rows {
		out = append(out, toModelQueryRow(row))
		totalCount = int(row.TotalCount)
	}

	return models.Page[models.Query]{
		Items:      out,
		TotalCount: totalCount,
		PageCount:  pageCountFor(totalCount, countPerPage),
	}, nil
}

func (c *Core) UpdateQuery(ctx context.Context, req models.UpdateQuery) (models.Query, error) {
	if err := validateQuery(req.SQL); err != nil {
		return models.Query{}, err
	}

	queryID, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.Query{}, fmt.Errorf("parse query uuid: %w", err)
	}

	q, err := c.store.UpdateQueryByUUID(ctx, repo.UpdateQueryByUUIDParams{
		Uuid:        queryID,
		Name:        req.Name,
		Sql:         req.SQL,
		Description: req.Description,
	})
	if err != nil {
		return models.Query{}, fmt.Errorf("update query: %w", err)
	}

	return toModelQuery(q), nil
}

func (c *Core) DeleteQuery(ctx context.Context, req models.ResourceID) error {
	queryID, err := uuid.Parse(req.UUID)
	if err != nil {
		return fmt.Errorf("parse query uuid: %w", err)
	}

	rows, err := c.store.DeleteQueryByUUID(ctx, queryID)
	if err != nil {
		return fmt.Errorf("delete query: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (c *Core) CreateSchedule(ctx context.Context, req models.CreateSchedule) (models.Schedule, error) {
	queryID, err := uuid.Parse(req.QueryUUID)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("parse query uuid: %w", err)
	}

	q, err := c.store.GetQueryByUUID(ctx, queryID)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("get query for schedule: %w", err)
	}

	params := repo.CreateScheduleParams{
		QueryID:         q.ID,
		Name:            req.Name,
		IntervalSeconds: int32(req.IntervalSeconds),
		Platform:        defaultString(req.Platform, "all"),
		Version:         req.Version,
		Shard:           int32(defaultInt(req.Shard, 100)),
		Denylist:        req.Denylist,
		Removed:         req.Removed,
		Snapshot:        req.Snapshot,
		Enabled:         defaultBool(req.Enabled, true),
		IsSystem:        req.IsSystem,
	}

	sched, err := c.store.CreateSchedule(ctx, params)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("create schedule: %w", err)
	}

	return toModelSchedule(sched, toModelQuery(q)), nil
}

func (c *Core) PaginateSchedules(ctx context.Context, req models.PageRequest) (models.Page[models.Schedule], error) {
	page := req.Page
	countPerPage := req.Count
	if countPerPage <= 0 {
		countPerPage = 10
	}
	if page < 0 {
		page = 0
	}

	rows, err := c.store.ListSchedulesWithQueries(ctx, repo.ListSchedulesWithQueriesParams{
		Limit:  int32(countPerPage),
		Offset: int32(page * countPerPage),
	})
	if err != nil {
		return models.Page[models.Schedule]{}, fmt.Errorf("list schedules: %w", err)
	}

	out := make([]models.Schedule, 0, len(rows))
	totalCount := 0
	for _, row := range rows {
		out = append(out, toModelScheduleRow(row))
		totalCount = int(row.TotalCount)
	}

	return models.Page[models.Schedule]{
		Items:      out,
		TotalCount: totalCount,
		PageCount:  pageCountFor(totalCount, countPerPage),
	}, nil
}

func (c *Core) ListEnabledSchedules(ctx context.Context, req models.ScheduleListRequest) ([]models.Schedule, error) {
	rows, err := c.store.ListEnabledSchedulesWithQueries(ctx, int32(req.Limit))
	if err != nil {
		return nil, fmt.Errorf("list enabled schedules: %w", err)
	}

	out := make([]models.Schedule, 0, len(rows))
	for _, row := range rows {
		out = append(out, toModelEnabledScheduleRow(row))
	}
	return out, nil
}

func (c *Core) GetSchedule(ctx context.Context, req models.ResourceID) (models.Schedule, error) {
	scheduleID, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("parse schedule uuid: %w", err)
	}

	sched, err := c.store.GetScheduleWithQueryByUUID(ctx, scheduleID)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("get schedule: %w", err)
	}

	return toModelScheduleWithQueryRow(sched), nil
}

func (c *Core) DeleteSchedule(ctx context.Context, req models.ResourceID) error {
	id, err := uuid.Parse(req.UUID)
	if err != nil {
		return fmt.Errorf("parse schedule uuid: %w", err)
	}

	rows, err := c.store.DeleteScheduleByUUID(ctx, id)
	if err != nil {
		return fmt.Errorf("delete schedule: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (c *Core) UpdateSchedule(ctx context.Context, req models.UpdateSchedule) (models.Schedule, error) {
	scheduleID, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("parse schedule uuid: %w", err)
	}

	queryID, err := uuid.Parse(req.QueryUUID)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("parse query uuid: %w", err)
	}

	q, err := c.store.GetQueryByUUID(ctx, queryID)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("get query for schedule: %w", err)
	}

	sched, err := c.store.UpdateScheduleByUUID(ctx, repo.UpdateScheduleByUUIDParams{
		Uuid:            scheduleID,
		QueryID:         q.ID,
		Name:            req.Name,
		IntervalSeconds: int32(req.IntervalSeconds),
		Platform:        defaultString(req.Platform, "all"),
		Version:         req.Version,
		Shard:           int32(defaultInt(req.Shard, 100)),
		Denylist:        req.Denylist,
		Removed:         req.Removed,
		Snapshot:        req.Snapshot,
		Enabled:         defaultBool(req.Enabled, true),
	})
	if err != nil {
		return models.Schedule{}, fmt.Errorf("update schedule: %w", err)
	}

	return toModelSchedule(sched, toModelQuery(q)), nil
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

	if err := c.store.TouchNode(ctx, nodeID); err != nil {
		return fmt.Errorf("touch node: %w", err)
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

func validateQuery(query string) error {
	if strings.TrimSpace(query) == "" {
		return fmt.Errorf("%w: query cannot be empty", ErrInvalidQuery)
	}

	keywords := []string{"SELECT", "FROM", "WHERE", "JOIN", "ORDER BY", "GROUP BY", "HAVING", "LIMIT"}
	upper := strings.ToUpper(query)
	for _, keyword := range keywords {
		if strings.Contains(upper, keyword) {
			return nil
		}
	}

	return fmt.Errorf("%w: query does not appear to be valid SQL", ErrInvalidQuery)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func defaultInt(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func defaultBool(value, fallback bool) bool {
	if value {
		return true
	}
	return fallback
}

func pageCountFor(total, perPage int) int {
	if total == 0 || perPage <= 0 {
		return 0
	}
	return int(math.Ceil(float64(total) / float64(perPage)))
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
