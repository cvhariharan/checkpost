package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/cvhariharan/checkpost/internal/repo"
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

	if node.OwnerUserUUID != uuid.Nil {
		if err := c.linkNodeOwner(ctx, created.ID, node.OwnerUserUUID); err != nil {
			c.logger.Warn("could not attribute enrolled node to owner", "node_id", created.ID, "error", err)
		}
	}

	return models.NodeCredentials{NodeKey: created.NodeKey.String()}, nil
}

func (c *Core) linkNodeOwner(ctx context.Context, nodeID int64, userUUID uuid.UUID) error {
	ownerID, err := c.resolveDeviceOwnerID(ctx, userUUID)
	if err != nil {
		return err
	}

	if err := c.store.LinkNodeInventoryOwnerIfUnassigned(ctx, repo.LinkNodeInventoryOwnerIfUnassignedParams{
		NodeID:  nodeID,
		OwnerID: sql.NullInt64{Int64: ownerID, Valid: true},
	}); err != nil {
		return fmt.Errorf("link node inventory owner: %w", err)
	}

	return nil
}

// resolveDeviceOwnerID maps a checkpost user to its device-owner row id,
// identifying the owner by email and creating it on first use
func (c *Core) resolveDeviceOwnerID(ctx context.Context, userUUID uuid.UUID) (int64, error) {
	user, err := c.store.GetUserByUUID(ctx, userUUID)
	if err != nil {
		return 0, fmt.Errorf("get user: %w", err)
	}

	email := strings.TrimSpace(firstNonEmpty(user.Email, user.Username))
	if email == "" {
		return 0, fmt.Errorf("user %s has no email to attribute ownership", userUUID)
	}

	id, err := c.store.UpsertDeviceOwnerByEmail(ctx, repo.UpsertDeviceOwnerByEmailParams{
		DisplayName: strings.TrimSpace(firstNonEmpty(user.Name, user.Username, email)),
		Email:       email,
	})
	if err != nil {
		return 0, fmt.Errorf("create device owner: %w", err)
	}
	return id, nil
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
	id, err := uuid.Parse(req.ID)
	if err != nil {
		return models.Node{}, fmt.Errorf("parse node uuid: %w", err)
	}

	node, err := c.store.GetNodeByUUID(ctx, id)
	if err != nil {
		return models.Node{}, fmt.Errorf("get node: %w", err)
	}

	out := toModelNode(node)
	out, err = c.attachGroupsToNode(ctx, out)
	if err != nil {
		return models.Node{}, err
	}
	out, err = c.attachInventoryToNode(ctx, out)
	if err != nil {
		return models.Node{}, err
	}
	out, err = c.attachComplianceScore(ctx, out, id)
	if err != nil {
		return models.Node{}, err
	}
	return out, nil
}

func (c *Core) attachComplianceScore(ctx context.Context, node models.Node, id uuid.UUID) (models.Node, error) {
	row, err := c.store.GetNodeComplianceScore(ctx, repo.GetNodeComplianceScoreParams{
		NodeUuid:    id,
		StaleCutoff: time.Now().UTC().Add(-c.policyStaleAfter),
	})
	if err != nil {
		return node, fmt.Errorf("get node compliance score: %w", err)
	}
	node.ComplianceScore = weightedComplianceScore(row.WeightedPassing, row.WeightedTotal)
	return node, nil
}

func (c *Core) UpdateNode(ctx context.Context, req models.UpdateNode) (models.Node, error) {
	id, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.Node{}, fmt.Errorf("parse node uuid: %w", err)
	}

	displayName := strings.TrimSpace(req.DisplayName)
	if utf8.RuneCountInString(displayName) > 255 {
		return models.Node{}, fmt.Errorf("%w: display name must be 255 characters or fewer", ErrInvalidNodeDisplayName)
	}
	for _, r := range displayName {
		if unicode.IsControl(r) {
			return models.Node{}, fmt.Errorf("%w: display name contains control characters", ErrInvalidNodeDisplayName)
		}
	}

	node, err := c.store.UpdateNodeDisplayNameByUUID(ctx, repo.UpdateNodeDisplayNameByUUIDParams{
		Uuid:        id,
		DisplayName: displayName,
	})
	if err != nil {
		return models.Node{}, fmt.Errorf("update node display name: %w", err)
	}

	out := toModelNode(node)
	out, err = c.attachGroupsToNode(ctx, out)
	if err != nil {
		return models.Node{}, err
	}
	out, err = c.attachInventoryToNode(ctx, out)
	if err != nil {
		return models.Node{}, err
	}
	return out, nil
}

func (c *Core) DeleteNode(ctx context.Context, req models.ResourceID) error {
	id, err := uuid.Parse(req.UUID)
	if err != nil {
		return fmt.Errorf("parse node uuid: %w", err)
	}

	rows, err := c.store.DeleteNodeByUUID(ctx, id)
	if err != nil {
		return fmt.Errorf("delete node: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (c *Core) PaginateNodes(ctx context.Context, req models.NodeListRequest) (models.Page[models.Node], error) {
	return c.paginateNodes(ctx, req, "")
}

func (c *Core) PaginateNodesForUser(ctx context.Context, userUUID string, req models.NodeListRequest) (models.Page[models.Node], error) {
	uid, err := uuid.Parse(userUUID)
	if err != nil {
		return models.Page[models.Node]{}, fmt.Errorf("parse user uuid: %w", err)
	}
	user, err := c.store.GetUserByUUID(ctx, uid)
	if err != nil {
		return models.Page[models.Node]{}, fmt.Errorf("get user: %w", err)
	}
	req.OwnerID = ""
	return c.paginateNodes(ctx, req, strings.TrimSpace(user.Email))
}

func (c *Core) paginateNodes(ctx context.Context, req models.NodeListRequest, ownerEmail string) (models.Page[models.Node], error) {
	page := req.Page
	countPerPage := req.Count
	if countPerPage <= 0 {
		countPerPage = 10
	}
	if page < 0 {
		page = 0
	}

	rows, err := c.store.ListNodes(ctx, repo.ListNodesParams{
		Query:       strings.TrimSpace(req.Query),
		Platform:    strings.TrimSpace(req.Platform),
		OwnerUuid:   strings.TrimSpace(req.OwnerID),
		Assigned:    strings.TrimSpace(req.Assigned),
		OwnerEmail:  strings.TrimSpace(ownerEmail),
		StaleCutoff: time.Now().UTC().Add(-c.policyStaleAfter),
		LimitCount:  int32(countPerPage),
		OffsetCount: int32(page * countPerPage),
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
