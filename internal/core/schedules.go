package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/cvhariharan/checkpost/internal/repo"
	"github.com/cvhariharan/checkpost/internal/resultquery"
	"github.com/cvhariharan/checkpost/internal/results"
	"github.com/google/uuid"
)

func (c *Core) CreateSchedule(ctx context.Context, req models.CreateSchedule) (models.Schedule, error) {
	if err := validateQuery(req.SQL); err != nil {
		return models.Schedule{}, err
	}

	groupUUIDs, err := parseUUIDList(req.GroupIDs, "group")
	if err != nil {
		return models.Schedule{}, err
	}

	platform := strings.TrimSpace(req.Platform)
	if platform == "" {
		platform = "all"
	}
	shard := req.Shard
	if shard == 0 {
		shard = 100
	}

	sched, err := c.store.CreateScheduleTx(ctx, repo.CreateScheduleTxParams{
		Schedule: repo.CreateScheduleParams{
			Name:            req.Name,
			Sql:             req.SQL,
			Description:     req.Description,
			IntervalSeconds: int32(req.IntervalSeconds),
			Platform:        platform,
			Version:         req.Version,
			Shard:           int32(shard),
			Denylist:        req.Denylist,
			Removed:         req.Removed,
			Snapshot:        req.Snapshot,
			Enabled:         req.Enabled,
			IsSystem:        req.IsSystem,
		},
		GroupUUIDs: groupUUIDs,
	})
	if err != nil {
		return models.Schedule{}, fmt.Errorf("create schedule: %w", err)
	}

	out := toModelSchedule(sched)
	out, err = c.attachGroupsToSchedule(ctx, out)
	if err != nil {
		return models.Schedule{}, err
	}
	return out, nil
}

// validateQuery is a best-effort sanity check that the inline schedule SQL
// looks like a query rather than empty or junk. osquery enforces the real
// grammar at runtime.
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

func (c *Core) ListScheduleResults(ctx context.Context, req models.ScheduleResultsRequest) (models.ScheduleResults, error) {
	if c.reader == nil {
		// Disable frontend browsing, no reader available
		return models.ScheduleResults{BrowsingDisabled: true}, nil
	}

	scheduleID, err := uuid.Parse(req.ScheduleUUID)
	if err != nil {
		return models.ScheduleResults{}, fmt.Errorf("parse schedule uuid: %w", err)
	}

	sched, err := c.store.GetScheduleByUUID(ctx, scheduleID)
	if err != nil {
		return models.ScheduleResults{}, fmt.Errorf("get schedule: %w", err)
	}

	count := req.Count
	if count <= 0 {
		count = 10
	}
	page := req.Page
	if page < 0 {
		page = 0
	}

	columns, err := c.loadScheduleColumns(ctx, sched.Uuid, sched.SqlVersion)
	if err != nil {
		return models.ScheduleResults{}, err
	}

	filters, nodeIDs, err := c.prepareResultFilters(ctx, req.Query, columns)
	if err != nil {
		return models.ScheduleResults{}, err
	}

	res, err := c.reader.Read(ctx, sched.Uuid, sched.SqlVersion, columns, results.ReadOptions{
		Snapshot: sched.Snapshot,
		Limit:    count,
		Offset:   page * count,
		Filters:  filters,
		NodeIDs:  nodeIDs,
	})
	if err != nil {
		return models.ScheduleResults{}, fmt.Errorf("read results: %w", err)
	}

	nodeNames, err := c.resolveNodeNames(ctx, res.Rows)
	if err != nil {
		return models.ScheduleResults{}, err
	}

	out := make([]models.ScheduleResultRow, 0, len(res.Rows))
	for _, row := range res.Rows {
		nu := nodeNames[row.NodeID]
		out = append(out, models.ScheduleResultRow{
			NodeUUID: nu.uuid,
			Hostname: nu.hostname,
			Columns:  row.Values,
			LastSeen: row.UnixTime,
		})
	}

	return models.ScheduleResults{
		Columns:      columns,
		Rows:         out,
		Total:        res.Total,
		Page:         page + 1,
		CountPerPage: count,
		PageCount:    pageCountFor(res.Total, count),
	}, nil
}

// prepareResultFilters parses the user's q parameter, validates it against
// the schedule's columns, and resolves any machine: terms to node IDs.
// Validation errors are wrapped as ErrInvalidQuery so the handler returns
// 400.
func (c *Core) prepareResultFilters(ctx context.Context, raw string, columns []string) ([]resultquery.Term, []int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil, nil
	}
	filters, err := resultquery.Parse(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidQuery, err)
	}
	if err := resultquery.Validate(filters, columns); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidQuery, err)
	}
	nodeIDs, err := c.resolveMachineFilters(ctx, filters)
	if err != nil {
		return nil, nil, err
	}
	return filters, nodeIDs, nil
}

// resolveMachineFilters expands every machine: term in filters to the
// matching node IDs. If at least one machine: term was present but no
// nodes matched, it returns a single-element [-1] sentinel so the reader's
// `node_id IN (...)` clause yields zero rows instead of being skipped.
func (c *Core) resolveMachineFilters(ctx context.Context, filters []resultquery.Term) ([]int64, error) {
	var (
		nodeIDs         []int64
		seenMachineTerm bool
	)
	for _, f := range filters {
		if f.Field != resultquery.FieldMachine {
			continue
		}
		seenMachineTerm = true
		matches, err := c.store.ListNodesMatchingIdentity(ctx, f.Value)
		if err != nil {
			return nil, fmt.Errorf("list matching machines: %w", err)
		}
		for _, m := range matches {
			nodeIDs = append(nodeIDs, m.ID)
		}
	}
	if seenMachineTerm && len(nodeIDs) == 0 {
		return []int64{-1}, nil
	}
	return nodeIDs, nil
}

type nodeIdentity struct {
	uuid     string
	hostname string
}

// resolveNodeNames bulk-resolves node IDs returned from DuckDB to UUID +
// hostname for the API response. Unknown IDs are omitted so the response
// renders a blank cell rather than misleading placeholder text.
func (c *Core) resolveNodeNames(ctx context.Context, rows []results.ResultRow) (map[int64]nodeIdentity, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	seen := make(map[int64]struct{}, len(rows))
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		if _, ok := seen[row.NodeID]; ok {
			continue
		}
		seen[row.NodeID] = struct{}{}
		ids = append(ids, row.NodeID)
	}
	hosts, err := c.store.ListNodesByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list nodes by ids: %w", err)
	}
	out := make(map[int64]nodeIdentity, len(hosts))
	for _, n := range hosts {
		out[n.ID] = nodeIdentity{uuid: n.Uuid.String(), hostname: n.Hostname}
	}
	return out, nil
}

// loadScheduleColumns reads the observed column list from query_schemas. If
// no schema has been recorded yet (a brand-new schedule that hasn't reported
// any rows), returns an empty slice.
func (c *Core) loadScheduleColumns(ctx context.Context, scheduleUUID uuid.UUID, sqlVersion int32) ([]string, error) {
	schema, err := c.store.GetQuerySchema(ctx, repo.GetQuerySchemaParams{
		ScheduleUuid: scheduleUUID,
		SqlVersion:   sqlVersion,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("get query schema: %w", err)
	}
	var cols []string
	if len(schema.Columns) > 0 {
		if err := json.Unmarshal(schema.Columns, &cols); err != nil {
			return nil, fmt.Errorf("decode columns: %w", err)
		}
	}
	return cols, nil
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

	rows, err := c.store.ListSchedules(ctx, repo.ListSchedulesParams{
		Query:       strings.TrimSpace(req.Query),
		LimitCount:  int32(countPerPage),
		OffsetCount: int32(page * countPerPage),
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

	if err := c.attachGroupsToSchedules(ctx, out); err != nil {
		return models.Page[models.Schedule]{}, err
	}

	return models.Page[models.Schedule]{
		Items:      out,
		TotalCount: totalCount,
		PageCount:  pageCountFor(totalCount, countPerPage),
	}, nil
}

func (c *Core) ListEnabledSchedules(ctx context.Context, req models.ScheduleListRequest) ([]models.Schedule, error) {
	rows, err := c.store.ListEnabledSchedules(ctx, int32(req.Limit))
	if err != nil {
		return nil, fmt.Errorf("list enabled schedules: %w", err)
	}

	out := make([]models.Schedule, 0, len(rows))
	for _, row := range rows {
		out = append(out, toModelSchedule(row))
	}
	return out, nil
}

func (c *Core) ListEnabledSchedulesForNode(ctx context.Context, req models.NodeKeyRequest, limit int) ([]models.Schedule, error) {
	node, err := c.GetNode(ctx, req)
	if err != nil {
		return nil, err
	}

	rows, err := c.store.ListEnabledSchedulesForNode(ctx, repo.ListEnabledSchedulesForNodeParams{
		NodeID:     node.ID,
		LimitCount: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list enabled schedules for node: %w", err)
	}

	out := make([]models.Schedule, 0, len(rows))
	for _, row := range rows {
		out = append(out, toModelSchedule(row))
	}
	return out, nil
}

func (c *Core) GetSchedule(ctx context.Context, req models.ResourceID) (models.Schedule, error) {
	scheduleID, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("parse schedule uuid: %w", err)
	}

	sched, err := c.store.GetScheduleByUUID(ctx, scheduleID)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("get schedule: %w", err)
	}

	out := toModelSchedule(sched)
	out, err = c.attachGroupsToSchedule(ctx, out)
	if err != nil {
		return models.Schedule{}, err
	}
	return out, nil
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

	// Clean up parquet partitions and query_schemas entries for this schedule.
	// Errors are logged, not returned: the schedule row is gone either way and
	// orphaned files will be swept by the schema GC.
	if c.sink != nil {
		if err := c.sink.DeleteSchedule(ctx, id); err != nil {
			c.logger.Error("remove schedule partitions", "schedule_uuid", id, "error", err)
		}
	}
	if err := c.store.DeleteQuerySchemasForSchedule(ctx, id); err != nil {
		c.logger.Error("remove query_schemas for schedule", "schedule_uuid", id, "error", err)
	}
	return nil
}

func (c *Core) UpdateSchedule(ctx context.Context, req models.UpdateSchedule) (models.Schedule, error) {
	scheduleID, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("parse schedule uuid: %w", err)
	}

	if err := validateQuery(req.SQL); err != nil {
		return models.Schedule{}, err
	}

	groupUUIDs, err := parseUUIDList(req.GroupIDs, "group")
	if err != nil {
		return models.Schedule{}, err
	}

	platform := strings.TrimSpace(req.Platform)
	if platform == "" {
		platform = "all"
	}
	shard := req.Shard
	if shard == 0 {
		shard = 100
	}

	sched, err := c.store.UpdateScheduleTx(ctx, repo.UpdateScheduleTxParams{
		Schedule: repo.UpdateScheduleByUUIDParams{
			Uuid:            scheduleID,
			Name:            req.Name,
			Sql:             req.SQL,
			Description:     req.Description,
			IntervalSeconds: int32(req.IntervalSeconds),
			Platform:        platform,
			Version:         req.Version,
			Shard:           int32(shard),
			Denylist:        req.Denylist,
			Removed:         req.Removed,
			Snapshot:        req.Snapshot,
			Enabled:         req.Enabled,
			RetentionDays:   int32(req.RetentionDays),
		},
		GroupUUIDs: groupUUIDs,
	})
	if err != nil {
		return models.Schedule{}, fmt.Errorf("update schedule: %w", err)
	}

	out := toModelSchedule(sched)
	out, err = c.attachGroupsToSchedule(ctx, out)
	if err != nil {
		return models.Schedule{}, err
	}
	return out, nil
}
