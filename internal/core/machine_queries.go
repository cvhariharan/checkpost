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
	"github.com/google/uuid"
)

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
	return nil
}

// ExecuteMachineQuery creates a adhoc query in the store with status as pending to be dispatched during the next node poll
func (c *Core) ExecuteMachineQuery(ctx context.Context, req models.MachineQueryRequest) (models.MachineQueryResult, error) {
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

	rowsForStorage := rows
	if rowsForStorage == nil && errMsg == "" {
		rowsForStorage = []interface{}{}
	}

	resultJSON, err := json.Marshal(rowsForStorage)
	if err != nil {
		return models.MachineQueryResult{}, true, fmt.Errorf("marshal ad-hoc query result: %w", err)
	}

	completed, err := c.store.CompleteMachineQueryResult(ctx, repo.CompleteMachineQueryResultParams{
		Uuid:    parsedID,
		Results: string(resultJSON),
		Error:   errMsg,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.logger.Debug("ignoring result for unknown ad-hoc query", "query_id", queryID)
			return models.MachineQueryResult{}, false, nil
		}
		return models.MachineQueryResult{}, true, fmt.Errorf("complete machine query result: %w", err)
	}

	return toModelMachineQueryResult(completed), true, nil
}
