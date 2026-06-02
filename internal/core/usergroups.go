package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/cvhariharan/checkpost/internal/repo"
	"github.com/google/uuid"
)

// CreateUserGroup creates a user group (an RBAC subject distinct from machine groups).
func (c *Core) CreateUserGroup(ctx context.Context, req models.CreateUserGroup) (models.UserGroup, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return models.UserGroup{}, errors.New("user group name cannot be empty")
	}
	group, err := c.store.CreateUserGroup(ctx, repo.CreateUserGroupParams{
		Name:           name,
		Description:    req.Description,
		OidcClaimValue: strings.TrimSpace(req.OIDCClaimValue),
	})
	if err != nil {
		return models.UserGroup{}, fmt.Errorf("create user group: %w", err)
	}
	return toModelUserGroup(group), nil
}

// ListUserGroups returns a paginated list of user groups.
func (c *Core) ListUserGroups(ctx context.Context, req models.PageRequest) (models.Page[models.UserGroup], error) {
	page := req.Page
	countPerPage := req.Count
	if countPerPage <= 0 {
		countPerPage = 10
	}
	if page < 0 {
		page = 0
	}

	rows, err := c.store.ListUserGroups(ctx, repo.ListUserGroupsParams{
		LimitCount:  int32(countPerPage),
		OffsetCount: int32(page * countPerPage),
	})
	if err != nil {
		return models.Page[models.UserGroup]{}, fmt.Errorf("list user groups: %w", err)
	}

	out := make([]models.UserGroup, 0, len(rows))
	totalCount := 0
	for _, row := range rows {
		out = append(out, models.UserGroup{
			ID:             row.ID,
			UUID:           row.Uuid.String(),
			Name:           row.Name,
			Description:    row.Description,
			OIDCClaimValue: row.OidcClaimValue,
			MemberCount:    int(row.MemberCount),
			CreatedAt:      row.CreatedAt,
			UpdatedAt:      row.UpdatedAt,
		})
		totalCount = int(row.TotalCount)
	}

	return models.Page[models.UserGroup]{
		Items:      out,
		TotalCount: totalCount,
		PageCount:  pageCountFor(totalCount, countPerPage),
	}, nil
}

// GetUserGroup returns a single user group by uuid.
func (c *Core) GetUserGroup(ctx context.Context, groupUUID string) (models.UserGroup, error) {
	gid, err := uuid.Parse(groupUUID)
	if err != nil {
		return models.UserGroup{}, fmt.Errorf("parse user group uuid: %w", err)
	}
	group, err := c.store.GetUserGroupByUUID(ctx, gid)
	if err != nil {
		return models.UserGroup{}, fmt.Errorf("get user group: %w", err)
	}
	return toModelUserGroup(group), nil
}

// UpdateUserGroup updates a user group's editable fields.
func (c *Core) UpdateUserGroup(ctx context.Context, req models.UpdateUserGroup) (models.UserGroup, error) {
	gid, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.UserGroup{}, fmt.Errorf("parse user group uuid: %w", err)
	}
	group, err := c.store.UpdateUserGroupByUUID(ctx, repo.UpdateUserGroupByUUIDParams{
		Uuid:           gid,
		Name:           strings.TrimSpace(req.Name),
		Description:    req.Description,
		OidcClaimValue: strings.TrimSpace(req.OIDCClaimValue),
	})
	if err != nil {
		return models.UserGroup{}, fmt.Errorf("update user group: %w", err)
	}
	return toModelUserGroup(group), nil
}

// DeleteUserGroup removes a user group; memberships and bindings cascade via FK.
func (c *Core) DeleteUserGroup(ctx context.Context, groupUUID string) error {
	gid, err := uuid.Parse(groupUUID)
	if err != nil {
		return fmt.Errorf("parse user group uuid: %w", err)
	}
	rows, err := c.store.DeleteUserGroupByUUID(ctx, gid)
	if err != nil {
		return fmt.Errorf("delete user group: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	// Rebuild policies so the deleted subject's grouping rows are dropped.
	return c.SyncPolicies(ctx)
}

// ListUserGroupMembers returns the users in a group.
func (c *Core) ListUserGroupMembers(ctx context.Context, groupUUID string) ([]models.UserGroupMember, error) {
	gid, err := uuid.Parse(groupUUID)
	if err != nil {
		return nil, fmt.Errorf("parse user group uuid: %w", err)
	}
	group, err := c.store.GetUserGroupByUUID(ctx, gid)
	if err != nil {
		return nil, fmt.Errorf("get user group: %w", err)
	}
	rows, err := c.store.ListUserGroupMembers(ctx, group.ID)
	if err != nil {
		return nil, fmt.Errorf("list user group members: %w", err)
	}
	out := make([]models.UserGroupMember, 0, len(rows))
	for _, row := range rows {
		out = append(out, models.UserGroupMember{
			UserUUID:  row.Uuid.String(),
			Username:  row.Username,
			Name:      row.Name,
			Email:     row.Email,
			LoginType: row.LoginType,
			Disabled:  row.Disabled,
			Source:    row.Source,
		})
	}
	return out, nil
}

// AddUserGroupMember adds a user to a group as a manual member (manual wins over oidc).
func (c *Core) AddUserGroupMember(ctx context.Context, groupUUID, userUUID string) error {
	groupID, userID, err := c.resolveGroupAndUser(ctx, groupUUID, userUUID)
	if err != nil {
		return err
	}
	if err := c.store.AddUserGroupMemberManual(ctx, repo.AddUserGroupMemberManualParams{
		UserGroupID: groupID,
		UserID:      userID,
	}); err != nil {
		return fmt.Errorf("add user group member: %w", err)
	}
	return nil
}

// RemoveUserGroupMember removes a user from a group.
func (c *Core) RemoveUserGroupMember(ctx context.Context, groupUUID, userUUID string) error {
	groupID, userID, err := c.resolveGroupAndUser(ctx, groupUUID, userUUID)
	if err != nil {
		return err
	}
	if err := c.store.RemoveUserGroupMember(ctx, repo.RemoveUserGroupMemberParams{
		UserGroupID: groupID,
		UserID:      userID,
	}); err != nil {
		return fmt.Errorf("remove user group member: %w", err)
	}
	return nil
}

func (c *Core) resolveGroupAndUser(ctx context.Context, groupUUID, userUUID string) (int64, int64, error) {
	gid, err := uuid.Parse(groupUUID)
	if err != nil {
		return 0, 0, fmt.Errorf("parse user group uuid: %w", err)
	}
	group, err := c.store.GetUserGroupByUUID(ctx, gid)
	if err != nil {
		return 0, 0, fmt.Errorf("get user group: %w", err)
	}
	uid, err := uuid.Parse(userUUID)
	if err != nil {
		return 0, 0, fmt.Errorf("parse user uuid: %w", err)
	}
	user, err := c.store.GetUserByUUID(ctx, uid)
	if err != nil {
		return 0, 0, fmt.Errorf("get user: %w", err)
	}
	return group.ID, user.ID, nil
}

// SyncUserGroupsFromClaims upserts source='oidc' memberships matching
// user_groups.oidc_claim_value for the supplied claim values, removes stale
// oidc-sourced memberships, and never touches source='manual' rows.
func (c *Core) SyncUserGroupsFromClaims(ctx context.Context, userID int64, claimGroups []string) error {
	claimValues := make([]string, 0, len(claimGroups))
	seen := map[string]struct{}{}
	for _, g := range claimGroups {
		v := strings.ToLower(strings.TrimSpace(g))
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		claimValues = append(claimValues, v)
	}

	var matched []repo.UserGroup
	if len(claimValues) > 0 {
		var err error
		matched, err = c.store.ListUserGroupsByClaimValues(ctx, claimValues)
		if err != nil {
			return fmt.Errorf("list user groups by claim: %w", err)
		}
	}

	keepIDs := make([]int64, 0, len(matched))
	for _, g := range matched {
		keepIDs = append(keepIDs, g.ID)
		if err := c.store.UpsertUserGroupMemberOIDC(ctx, repo.UpsertUserGroupMemberOIDCParams{
			UserGroupID: g.ID,
			UserID:      userID,
		}); err != nil {
			return fmt.Errorf("upsert oidc member: %w", err)
		}
	}

	if len(keepIDs) == 0 {
		if err := c.store.DeleteAllOIDCMembersForUser(ctx, userID); err != nil {
			return fmt.Errorf("delete oidc members: %w", err)
		}
		return nil
	}

	if err := c.store.DeleteStaleOIDCMembers(ctx, repo.DeleteStaleOIDCMembersParams{
		UserID:       userID,
		KeepGroupIds: keepIDs,
	}); err != nil {
		return fmt.Errorf("delete stale oidc members: %w", err)
	}
	return nil
}
