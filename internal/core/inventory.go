package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/cvhariharan/checkpost/internal/repo"
	"github.com/google/uuid"
)

func (c *Core) CreateDeviceOwner(ctx context.Context, req models.CreateDeviceOwner) (models.DeviceOwner, error) {
	owner, err := c.store.CreateDeviceOwner(ctx, repo.CreateDeviceOwnerParams{
		DisplayName: strings.TrimSpace(req.DisplayName),
		Email:       strings.TrimSpace(req.Email),
		ExternalID:  strings.TrimSpace(req.ExternalID),
		Department:  strings.TrimSpace(req.Department),
		Title:       strings.TrimSpace(req.Title),
		Phone:       strings.TrimSpace(req.Phone),
		Notes:       strings.TrimSpace(req.Notes),
	})
	if err != nil {
		return models.DeviceOwner{}, fmt.Errorf("create owner: %w", err)
	}

	return toModelDeviceOwner(owner), nil
}

func (c *Core) GetDeviceOwner(ctx context.Context, req models.ResourceID) (models.DeviceOwner, error) {
	ownerID, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.DeviceOwner{}, fmt.Errorf("parse owner uuid: %w", err)
	}

	owner, err := c.store.GetDeviceOwnerWithCountsByUUID(ctx, ownerID)
	if err != nil {
		return models.DeviceOwner{}, fmt.Errorf("get owner: %w", err)
	}

	return toModelDeviceOwnerCountRow(owner), nil
}

func (c *Core) PaginateDeviceOwners(ctx context.Context, req models.DeviceOwnerListRequest) (models.Page[models.DeviceOwner], error) {
	page := req.Page
	countPerPage := req.Count
	if countPerPage <= 0 {
		countPerPage = 10
	}
	if page < 0 {
		page = 0
	}

	rows, err := c.store.ListDeviceOwnersWithCounts(ctx, repo.ListDeviceOwnersWithCountsParams{
		Query:       strings.TrimSpace(req.Query),
		LimitCount:  int32(countPerPage),
		OffsetCount: int32(page * countPerPage),
	})
	if err != nil {
		return models.Page[models.DeviceOwner]{}, fmt.Errorf("list owners: %w", err)
	}

	out := make([]models.DeviceOwner, 0, len(rows))
	totalCount := 0
	for _, row := range rows {
		out = append(out, toModelDeviceOwnerRow(row))
		totalCount = int(row.TotalCount)
	}

	return models.Page[models.DeviceOwner]{
		Items:      out,
		TotalCount: totalCount,
		PageCount:  pageCountFor(totalCount, countPerPage),
	}, nil
}

func (c *Core) UpdateDeviceOwner(ctx context.Context, req models.UpdateDeviceOwner) (models.DeviceOwner, error) {
	ownerID, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.DeviceOwner{}, fmt.Errorf("parse owner uuid: %w", err)
	}

	owner, err := c.store.UpdateDeviceOwnerByUUID(ctx, repo.UpdateDeviceOwnerByUUIDParams{
		Uuid:        ownerID,
		DisplayName: strings.TrimSpace(req.DisplayName),
		Email:       strings.TrimSpace(req.Email),
		ExternalID:  strings.TrimSpace(req.ExternalID),
		Department:  strings.TrimSpace(req.Department),
		Title:       strings.TrimSpace(req.Title),
		Phone:       strings.TrimSpace(req.Phone),
		Notes:       strings.TrimSpace(req.Notes),
	})
	if err != nil {
		return models.DeviceOwner{}, fmt.Errorf("update owner: %w", err)
	}

	return toModelDeviceOwner(owner), nil
}

func (c *Core) DeleteDeviceOwner(ctx context.Context, req models.ResourceID) error {
	ownerID, err := uuid.Parse(req.UUID)
	if err != nil {
		return fmt.Errorf("parse owner uuid: %w", err)
	}

	rows, err := c.store.DeleteDeviceOwnerByUUID(ctx, ownerID)
	if err != nil {
		return fmt.Errorf("delete owner: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (c *Core) GetNodeInventory(ctx context.Context, req models.NodeIdentity) (models.NodeInventory, error) {
	nodeID, err := uuid.Parse(req.ID)
	if err != nil {
		return models.NodeInventory{}, fmt.Errorf("parse node uuid: %w", err)
	}

	if _, err := c.store.GetNodeByUUID(ctx, nodeID); err != nil {
		return models.NodeInventory{}, fmt.Errorf("get node: %w", err)
	}

	row, err := c.store.GetNodeInventoryByNodeUUID(ctx, nodeID)
	if err != nil {
		if err == sql.ErrNoRows {
			return models.NodeInventory{}, nil
		}
		return models.NodeInventory{}, fmt.Errorf("get node inventory: %w", err)
	}

	return toModelNodeInventory(row), nil
}

func (c *Core) UpdateNodeInventory(ctx context.Context, req models.UpdateNodeInventory) (models.NodeInventory, error) {
	nodeID, err := uuid.Parse(req.NodeUUID)
	if err != nil {
		return models.NodeInventory{}, fmt.Errorf("parse node uuid: %w", err)
	}

	var ownerID *uuid.UUID
	if strings.TrimSpace(req.OwnerUUID) != "" {
		parsed, err := uuid.Parse(req.OwnerUUID)
		if err != nil {
			return models.NodeInventory{}, fmt.Errorf("parse owner uuid: %w", err)
		}
		ownerID = &parsed
	}

	row, err := c.store.UpdateNodeInventoryTx(ctx, repo.UpdateNodeInventoryTxParams{
		NodeUUID:           nodeID,
		OwnerUUID:          ownerID,
		InternalTrackingID: strings.TrimSpace(req.InternalTrackingID),
		Notes:              strings.TrimSpace(req.Notes),
	})
	if err != nil {
		return models.NodeInventory{}, fmt.Errorf("update node inventory: %w", err)
	}

	return toModelNodeInventory(row), nil
}

func (c *Core) DeleteNodeInventory(ctx context.Context, req models.NodeIdentity) error {
	nodeID, err := uuid.Parse(req.ID)
	if err != nil {
		return fmt.Errorf("parse node uuid: %w", err)
	}

	if _, err := c.store.GetNodeByUUID(ctx, nodeID); err != nil {
		return fmt.Errorf("get node: %w", err)
	}

	if _, err := c.store.DeleteNodeInventoryByNodeUUID(ctx, nodeID); err != nil {
		return fmt.Errorf("delete node inventory: %w", err)
	}
	return nil
}

func (c *Core) PaginateOwnerMachines(ctx context.Context, req models.OwnerMachinesRequest) (models.Page[models.Node], error) {
	ownerID, err := uuid.Parse(req.OwnerUUID)
	if err != nil {
		return models.Page[models.Node]{}, fmt.Errorf("parse owner uuid: %w", err)
	}

	if _, err := c.store.GetDeviceOwnerByUUID(ctx, ownerID); err != nil {
		return models.Page[models.Node]{}, fmt.Errorf("get owner: %w", err)
	}

	page := req.Page
	countPerPage := req.Count
	if countPerPage <= 0 {
		countPerPage = 10
	}
	if page < 0 {
		page = 0
	}

	rows, err := c.store.ListNodesByOwner(ctx, repo.ListNodesByOwnerParams{
		OwnerUuid:   ownerID,
		LimitCount:  int32(countPerPage),
		OffsetCount: int32(page * countPerPage),
	})
	if err != nil {
		return models.Page[models.Node]{}, fmt.Errorf("list owner machines: %w", err)
	}

	out := make([]models.Node, 0, len(rows))
	totalCount := 0
	for _, row := range rows {
		out = append(out, toModelNodeFromOwnerRow(row))
		totalCount = int(row.TotalCount)
	}

	if err := c.attachGroupsToNodes(ctx, out); err != nil {
		return models.Page[models.Node]{}, err
	}
	if err := c.attachInventoryToNodes(ctx, out); err != nil {
		return models.Page[models.Node]{}, err
	}

	return models.Page[models.Node]{
		Items:      out,
		TotalCount: totalCount,
		PageCount:  pageCountFor(totalCount, countPerPage),
	}, nil
}

func (c *Core) attachInventoryToNode(ctx context.Context, node models.Node) (models.Node, error) {
	if strings.TrimSpace(node.UUID) == "" {
		return node, nil
	}

	inventory, err := c.GetNodeInventory(ctx, models.NodeIdentity{ID: node.UUID})
	if err != nil {
		return node, err
	}
	if !inventory.CreatedAt.IsZero() {
		node.Inventory = &inventory
	}
	return node, nil
}

func (c *Core) attachInventoryToNodes(ctx context.Context, nodes []models.Node) error {
	if len(nodes) == 0 {
		return nil
	}

	nodeUUIDs := make([]uuid.UUID, 0, len(nodes))
	indexByUUID := make(map[uuid.UUID]int, len(nodes))
	for i := range nodes {
		nodeID, err := uuid.Parse(nodes[i].UUID)
		if err != nil {
			return fmt.Errorf("parse node uuid: %w", err)
		}
		nodeUUIDs = append(nodeUUIDs, nodeID)
		indexByUUID[nodeID] = i
	}

	rows, err := c.store.ListNodeInventoriesByNodeUUIDs(ctx, nodeUUIDs)
	if err != nil {
		return fmt.Errorf("list node inventories: %w", err)
	}

	for _, row := range rows {
		idx, ok := indexByUUID[row.NodeUuid]
		if !ok {
			continue
		}
		inventory := toModelNodeInventoryListRow(row)
		nodes[idx].Inventory = &inventory
	}

	return nil
}
