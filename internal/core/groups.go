package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/cvhariharan/watcher/internal/models"
	"github.com/cvhariharan/watcher/internal/repo"
	"github.com/google/uuid"
)

func (c *Core) CreateGroup(ctx context.Context, req models.CreateGroup) (models.Group, error) {
	group, err := c.store.CreateGroup(ctx, repo.CreateGroupParams{
		Name:        strings.TrimSpace(req.Name),
		Description: req.Description,
	})
	if err != nil {
		return models.Group{}, fmt.Errorf("create group: %w", err)
	}

	return toModelGroup(group), nil
}

func (c *Core) GetGroup(ctx context.Context, req models.ResourceID) (models.Group, error) {
	groupID, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.Group{}, fmt.Errorf("parse group uuid: %w", err)
	}

	group, err := c.store.GetGroupWithCountsByUUID(ctx, groupID)
	if err != nil {
		return models.Group{}, fmt.Errorf("get group: %w", err)
	}

	return toModelGroupCountRow(group), nil
}

func (c *Core) PaginateGroups(ctx context.Context, req models.PageRequest) (models.Page[models.Group], error) {
	page := req.Page
	countPerPage := req.Count
	if countPerPage <= 0 {
		countPerPage = 10
	}
	if page < 0 {
		page = 0
	}

	rows, err := c.store.ListGroupsWithCounts(ctx, repo.ListGroupsWithCountsParams{
		LimitCount:  int32(countPerPage),
		OffsetCount: int32(page * countPerPage),
	})
	if err != nil {
		return models.Page[models.Group]{}, fmt.Errorf("list groups: %w", err)
	}

	out := make([]models.Group, 0, len(rows))
	totalCount := 0
	for _, row := range rows {
		out = append(out, toModelGroupRow(row))
		totalCount = int(row.TotalCount)
	}

	return models.Page[models.Group]{
		Items:      out,
		TotalCount: totalCount,
		PageCount:  pageCountFor(totalCount, countPerPage),
	}, nil
}

func (c *Core) UpdateGroup(ctx context.Context, req models.UpdateGroup) (models.Group, error) {
	groupID, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.Group{}, fmt.Errorf("parse group uuid: %w", err)
	}

	group, err := c.store.UpdateGroupByUUID(ctx, repo.UpdateGroupByUUIDParams{
		Uuid:        groupID,
		Name:        strings.TrimSpace(req.Name),
		Description: req.Description,
	})
	if err != nil {
		return models.Group{}, fmt.Errorf("update group: %w", err)
	}

	return toModelGroup(group), nil
}

func (c *Core) DeleteGroup(ctx context.Context, req models.ResourceID) error {
	groupID, err := uuid.Parse(req.UUID)
	if err != nil {
		return fmt.Errorf("parse group uuid: %w", err)
	}

	rows, err := c.store.DeleteGroupByUUID(ctx, groupID)
	if err != nil {
		return fmt.Errorf("delete group: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (c *Core) ListGroupsForNode(ctx context.Context, req models.NodeIdentity) ([]models.Group, error) {
	nodeID, err := uuid.Parse(req.ID)
	if err != nil {
		return nil, fmt.Errorf("parse node uuid: %w", err)
	}

	rows, err := c.store.ListGroupsForNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("list node groups: %w", err)
	}

	out := make([]models.Group, 0, len(rows))
	for _, row := range rows {
		out = append(out, toModelGroup(row))
	}
	return out, nil
}

func (c *Core) ReplaceGroupsForNode(ctx context.Context, req models.NodeIdentity, groupIDs []string) ([]models.Group, error) {
	nodeID, err := uuid.Parse(req.ID)
	if err != nil {
		return nil, fmt.Errorf("parse node uuid: %w", err)
	}

	groupUUIDs, err := parseUUIDList(groupIDs, "group")
	if err != nil {
		return nil, err
	}

	if err := c.store.ReplaceNodeGroupsTx(ctx, repo.ReplaceNodeGroupsTxParams{
		NodeUUID:   nodeID,
		GroupUUIDs: groupUUIDs,
	}); err != nil {
		return nil, fmt.Errorf("replace node groups: %w", err)
	}

	return c.ListGroupsForNode(ctx, req)
}

func (c *Core) PaginateGroupMachines(ctx context.Context, req models.GroupMachinesRequest) (models.Page[models.Node], error) {
	groupID, err := uuid.Parse(req.GroupUUID)
	if err != nil {
		return models.Page[models.Node]{}, fmt.Errorf("parse group uuid: %w", err)
	}

	page := req.Page
	countPerPage := req.Count
	if countPerPage <= 0 {
		countPerPage = 10
	}
	if page < 0 {
		page = 0
	}

	rows, err := c.store.ListNodesByGroup(ctx, repo.ListNodesByGroupParams{
		GroupUuid:   groupID,
		LimitCount:  int32(countPerPage),
		OffsetCount: int32(page * countPerPage),
	})
	if err != nil {
		return models.Page[models.Node]{}, fmt.Errorf("list group machines: %w", err)
	}

	out := make([]models.Node, 0, len(rows))
	totalCount := 0
	for _, row := range rows {
		out = append(out, toModelNodeFromGroupRow(row))
		totalCount = int(row.TotalCount)
	}

	if err := c.attachGroupsToNodes(ctx, out); err != nil {
		return models.Page[models.Node]{}, err
	}

	return models.Page[models.Node]{
		Items:      out,
		TotalCount: totalCount,
		PageCount:  pageCountFor(totalCount, countPerPage),
	}, nil
}

func (c *Core) attachGroupsToNode(ctx context.Context, node *models.Node) error {
	if node == nil || strings.TrimSpace(node.UUID) == "" {
		return nil
	}

	groups, err := c.ListGroupsForNode(ctx, models.NodeIdentity{ID: node.UUID})
	if err != nil {
		return err
	}
	node.Groups = groups
	return nil
}

func (c *Core) attachGroupsToNodes(ctx context.Context, nodes []models.Node) error {
	for i := range nodes {
		if err := c.attachGroupsToNode(ctx, &nodes[i]); err != nil {
			return err
		}
	}
	return nil
}

func (c *Core) attachGroupsToPolicy(ctx context.Context, policy *models.Policy) error {
	if policy == nil || strings.TrimSpace(policy.UUID) == "" {
		return nil
	}

	policyID, err := uuid.Parse(policy.UUID)
	if err != nil {
		return fmt.Errorf("parse policy uuid: %w", err)
	}

	rows, err := c.store.ListGroupsForPolicy(ctx, policyID)
	if err != nil {
		return fmt.Errorf("list policy groups: %w", err)
	}

	groups := make([]models.Group, 0, len(rows))
	for _, row := range rows {
		groups = append(groups, toModelGroup(row))
	}
	policy.Groups = groups
	policy.TargetAllMachines = len(groups) == 0
	return nil
}

func (c *Core) attachGroupsToPolicies(ctx context.Context, policies []models.Policy) error {
	for i := range policies {
		if err := c.attachGroupsToPolicy(ctx, &policies[i]); err != nil {
			return err
		}
	}
	return nil
}

func parseUUIDList(values []string, label string) ([]uuid.UUID, error) {
	if len(values) == 0 {
		return nil, nil
	}

	out := make([]uuid.UUID, 0, len(values))
	seen := make(map[uuid.UUID]struct{}, len(values))
	for _, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		parsed, err := uuid.Parse(trimmed)
		if err != nil {
			return nil, fmt.Errorf("parse %s uuid %q: %w", label, raw, err)
		}
		if _, ok := seen[parsed]; ok {
			continue
		}
		seen[parsed] = struct{}{}
		out = append(out, parsed)
	}
	return out, nil
}
