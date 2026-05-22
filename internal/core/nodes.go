package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/cvhariharan/watcher/internal/models"
	"github.com/cvhariharan/watcher/internal/repo"
	"github.com/google/uuid"
)

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

func (c *Core) RecordNodeHeartbeat(ctx context.Context, req models.NodeKeyRequest) error {
	id, err := uuid.Parse(req.NodeKey)
	if err != nil {
		return fmt.Errorf("parse node key: %w", err)
	}

	if err := c.store.TouchNode(ctx, id); err != nil {
		return fmt.Errorf("touch node: %w", err)
	}

	return nil
}

func (c *Core) GetNodeByID(ctx context.Context, req models.NodeIdentity) (models.Node, error) {
	if strings.TrimSpace(req.ID) == "" {
		return models.Node{}, fmt.Errorf("node id cannot be empty")
	}

	id, err := uuid.Parse(req.ID)
	if err != nil {
		return models.Node{}, fmt.Errorf("parse node uuid: %w", err)
	}

	node, err := c.store.GetNodeByUUID(ctx, id)
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
