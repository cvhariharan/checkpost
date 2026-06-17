package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/cvhariharan/checkpost/internal/repo"
	"github.com/cvhariharan/checkpost/internal/results"
	"github.com/google/uuid"
)

// adHocSQLVersion is fixed: ad-hoc queries are not versioned like schedules.
const adHocSQLVersion = 1

func (c *Core) ListMachineQueries(ctx context.Context, req models.NodeIdentity, pageReq models.PageRequest) (models.Page[models.MachineQueryResult], error) {
	node, err := c.GetNodeByID(ctx, req)
	if err != nil {
		return models.Page[models.MachineQueryResult]{}, err
	}

	nodeUUID, err := uuid.Parse(node.UUID)
	if err != nil {
		return models.Page[models.MachineQueryResult]{}, fmt.Errorf("parse node uuid: %w", err)
	}

	page := pageReq.Page
	countPerPage := pageReq.Count
	if countPerPage <= 0 {
		countPerPage = 10
	}
	if page < 0 {
		page = 0
	}

	rows, err := c.store.ListMachineQueryResultsByNodeUUID(ctx, repo.ListMachineQueryResultsByNodeUUIDParams{
		NodeUuid:    nodeUUID,
		LimitCount:  int32(countPerPage),
		OffsetCount: int32(page * countPerPage),
	})
	if err != nil {
		return models.Page[models.MachineQueryResult]{}, fmt.Errorf("list machine query results: %w", err)
	}

	out := make([]models.MachineQueryResult, 0, len(rows))
	totalCount := 0
	for _, row := range rows {
		out = append(out, toModelMachineQueryResultRow(row))
		totalCount = int(row.TotalCount)
	}
	return models.Page[models.MachineQueryResult]{
		Items:      out,
		TotalCount: totalCount,
		PageCount:  pageCountFor(totalCount, countPerPage),
	}, nil
}

// GetAdHocQueryResults returns one page of result rows; browsing is disabled
// when no reader is configured.
func (c *Core) GetAdHocQueryResults(ctx context.Context, queryUUID string, page, count int) (models.AdHocQueryResults, error) {
	if c.reader == nil {
		return models.AdHocQueryResults{BrowsingDisabled: true}, nil
	}

	qUUID, err := uuid.Parse(queryUUID)
	if err != nil {
		return models.AdHocQueryResults{}, fmt.Errorf("parse query uuid: %w", err)
	}

	rec, err := c.store.GetMachineQueryResultByUUID(ctx, qUUID)
	if err != nil {
		return models.AdHocQueryResults{}, fmt.Errorf("get machine query result: %w", err)
	}
	if rec.Status == "pending" {
		return models.AdHocQueryResults{Pending: true}, nil
	}
	if rec.Error != "" {
		return models.AdHocQueryResults{Error: rec.Error}, nil
	}

	if count <= 0 {
		count = 10
	}
	if page < 0 {
		page = 0
	}

	columns, err := c.loadResultColumns(ctx, qUUID, adHocSQLVersion)
	if err != nil {
		return models.AdHocQueryResults{}, err
	}

	res, err := c.reader.Read(ctx, qUUID, adHocSQLVersion, columns, results.ReadOptions{
		Snapshot: true,
		Limit:    count,
		Offset:   page * count,
	})
	if err != nil {
		return models.AdHocQueryResults{}, fmt.Errorf("read ad-hoc results: %w", err)
	}

	rows := make([]map[string]string, 0, len(res.Rows))
	for _, r := range res.Rows {
		rows = append(rows, r.Values)
	}
	return models.AdHocQueryResults{
		Columns:      res.Columns,
		Rows:         rows,
		Total:        res.Total,
		Page:         page + 1,
		CountPerPage: count,
		PageCount:    pageCountFor(res.Total, count),
	}, nil
}

func (c *Core) DeleteMachineQuery(ctx context.Context, nodeReq models.NodeIdentity, queryReq models.ResourceID) error {
	node, err := c.GetNodeByID(ctx, nodeReq)
	if err != nil {
		return err
	}

	nodeUUID, err := uuid.Parse(node.UUID)
	if err != nil {
		return fmt.Errorf("parse node uuid: %w", err)
	}

	queryUUID, err := uuid.Parse(queryReq.UUID)
	if err != nil {
		return fmt.Errorf("parse query uuid: %w", err)
	}

	rows, err := c.store.DeleteMachineQueryResultByNodeAndUUID(ctx, repo.DeleteMachineQueryResultByNodeAndUUIDParams{
		NodeUuid:  nodeUUID,
		QueryUuid: queryUUID,
	})
	if err != nil {
		return fmt.Errorf("delete machine query result: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	c.deleteResultSource(ctx, queryUUID)
	return nil
}

// deleteResultSource is best-effort: the lifecycle row is already gone and the
// TTL sweep is the backstop.
func (c *Core) deleteResultSource(ctx context.Context, sourceUUID uuid.UUID) {
	if c.sink != nil {
		if err := c.sink.Delete(ctx, sourceUUID); err != nil {
			c.logger.Error("remove ad-hoc result partitions", "source_uuid", sourceUUID, "error", err)
		}
	}
	if err := c.store.DeleteQuerySchemasForSource(ctx, sourceUUID); err != nil {
		c.logger.Error("remove query_schemas for ad-hoc result", "source_uuid", sourceUUID, "error", err)
	}
}

// ExecuteMachineQuery creates a adhoc query in the store with status as pending to be dispatched during the next node poll
func (c *Core) ExecuteMachineQuery(ctx context.Context, req models.MachineQueryRequest) (models.MachineQueryResult, error) {
	if c.sink == nil {
		return models.MachineQueryResult{}, ErrResultsBackendDisabled
	}

	query := strings.TrimSpace(req.Query)
	if err := validateQuery(query); err != nil {
		return models.MachineQueryResult{}, err
	}

	node, err := c.GetNodeByID(ctx, models.NodeIdentity{ID: req.NodeUUID})
	if err != nil {
		return models.MachineQueryResult{}, err
	}

	queryID := uuid.New()
	created, err := c.store.CreateMachineQueryResult(ctx, repo.CreateMachineQueryResultParams{
		Uuid:   queryID,
		NodeID: node.ID,
		Query:  query,
		RunID:  sql.NullInt64{},
	})
	if err != nil {
		return models.MachineQueryResult{}, fmt.Errorf("create machine query result: %w", err)
	}

	return toModelMachineQueryResult(created), nil
}

func (c *Core) pendingMachineQueries(ctx context.Context, node models.Node) ([]models.MachineQueryResult, error) {
	rows, err := c.store.ListPendingMachineQueryResults(ctx, node.ID)
	if err != nil {
		return nil, fmt.Errorf("list pending machine query results: %w", err)
	}

	ids := make([]int64, 0, len(rows))
	out := make([]models.MachineQueryResult, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
		out = append(out, toModelMachineQueryResult(row))
	}

	if err := c.store.MarkMachineQueryResultsDispatched(ctx, ids); err != nil {
		return nil, fmt.Errorf("mark machine query results dispatched: %w", err)
	}

	return out, nil
}

// completeAdHocResult parses the query response and stores the json results in the DB. Used for adhoc query results.
func (c *Core) completeAdHocResult(ctx context.Context, queryID string, rows interface{}, errMsg string) (models.MachineQueryResult, bool, error) {
	parsedID, err := uuid.Parse(queryID)
	if err != nil {
		c.logger.Debug("ignoring malformed ad-hoc query id", "query_id", queryID, "error", err)
		return models.MachineQueryResult{}, false, nil
	}

	resultRows := toResultRows(rows)

	completed, err := c.store.CompleteMachineQueryResult(ctx, repo.CompleteMachineQueryResultParams{
		Uuid:     parsedID,
		RowCount: int32(len(resultRows)),
		Error:    errMsg,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.logger.Debug("ignoring result for unknown ad-hoc query", "query_id", queryID)
			return models.MachineQueryResult{}, false, nil
		}
		return models.MachineQueryResult{}, true, fmt.Errorf("complete machine query result: %w", err)
	}

	if c.sink != nil && errMsg == "" && len(resultRows) > 0 {
		for i := range resultRows {
			resultRows[i].NodeID = completed.NodeID
		}
		if err := c.sink.Submit(ctx, results.Batch{
			SourceUUID: parsedID,
			SQLVersion: adHocSQLVersion,
			SourceName: "adhoc",
			Kind:       results.KindAdhoc,
			Snapshot:   true,
			Rows:       resultRows,
		}); err != nil {
			c.logger.Error("submit ad-hoc query result", "query_id", queryID, "error", err)
		}
	}

	return toModelMachineQueryResult(completed), true, nil
}

// toResultRows converts an ad-hoc query payload into snapshot rows. NodeID is
// set by the caller after the lifecycle row is resolved.
func toResultRows(rows interface{}) []results.Row {
	arr, ok := rows.([]interface{})
	if !ok {
		return nil
	}
	ut := time.Now().UTC()
	out := make([]results.Row, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		out = append(out, resultRowFromMap(m, "snapshot", 0, ut, ""))
	}
	return out
}
