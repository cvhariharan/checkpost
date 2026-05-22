package core

import (
	"context"
	"time"

	"github.com/cvhariharan/watcher/internal/models"
	"github.com/google/uuid"
)

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
