package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"

	"github.com/casbin/casbin/v2"
	casbinmodel "github.com/casbin/casbin/v2/model"
	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/cvhariharan/checkpost/internal/repo"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	sqlxadapter "github.com/memwey/casbin-sqlx-adapter"
)

const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleAnalyst  = "analyst"
	RoleViewer   = "viewer"
)

const (
	ResourceMachine      = "machine"
	ResourceMachineGroup = "machine_group"
	ResourcePolicy       = "policy"
	ResourceSchedule     = "schedule"
	ResourceQueryResult  = "query_result"
	ResourceYaraSource   = "yara_source"
	ResourceYaraScan     = "yara_scan"
	ResourceInventory    = "inventory"
	ResourceUser         = "user"
	ResourceUserGroup    = "user_group"
	ResourceRoleBinding  = "role_binding"
	ResourceSetting      = "setting"
	ResourceAlertRule    = "alert_rule"
	ResourceAlertTarget  = "alert_target"
)

const (
	ActionView    = "view"
	ActionCreate  = "create"
	ActionUpdate  = "update"
	ActionDelete  = "delete"
	ActionExecute = "execute"
)

const (
	SubjectUser      = "user"
	SubjectUserGroup = "usergroup"
)

const globalDomain = "*"

var ErrInvalidRole = errors.New("invalid role")

// permissionCatalog is the static resource -> (actions, scopable) catalog.
var permissionCatalog = []models.PermissionCatalogEntry{
	{Resource: ResourceMachine, Actions: []string{ActionView, ActionUpdate, ActionExecute, ActionDelete}, Scopable: true},
	{Resource: ResourceMachineGroup, Actions: []string{ActionView, ActionCreate, ActionUpdate, ActionDelete}, Scopable: true},
	{Resource: ResourcePolicy, Actions: []string{ActionView, ActionCreate, ActionUpdate, ActionDelete}, Scopable: true},
	{Resource: ResourceSchedule, Actions: []string{ActionView, ActionCreate, ActionUpdate, ActionDelete}, Scopable: true},
	{Resource: ResourceQueryResult, Actions: []string{ActionView, ActionDelete}, Scopable: true},
	{Resource: ResourceYaraSource, Actions: []string{ActionView, ActionCreate, ActionUpdate, ActionDelete}, Scopable: true},
	{Resource: ResourceYaraScan, Actions: []string{ActionView, ActionCreate}, Scopable: true},
	{Resource: ResourceInventory, Actions: []string{ActionView, ActionCreate, ActionUpdate, ActionDelete}, Scopable: true},
	{Resource: ResourceUser, Actions: []string{ActionView, ActionCreate, ActionUpdate, ActionDelete}, Scopable: false},
	{Resource: ResourceUserGroup, Actions: []string{ActionView, ActionCreate, ActionUpdate, ActionDelete}, Scopable: false},
	{Resource: ResourceRoleBinding, Actions: []string{ActionView, ActionCreate, ActionDelete}, Scopable: false},
	{Resource: ResourceSetting, Actions: []string{ActionView, ActionUpdate}, Scopable: false},
	{Resource: ResourceAlertRule, Actions: []string{ActionView, ActionCreate, ActionUpdate, ActionDelete}, Scopable: false},
	{Resource: ResourceAlertTarget, Actions: []string{ActionView, ActionCreate, ActionUpdate, ActionDelete, ActionExecute}, Scopable: false},
}

// roleMatrix is set for each built-in role:
// role -> resource -> actions. Seeded into Casbin.
var roleMatrix = map[string]map[string][]string{
	RoleAdmin: {
		ResourceMachine:      {ActionView, ActionUpdate, ActionExecute, ActionDelete},
		ResourceMachineGroup: {ActionView, ActionCreate, ActionUpdate, ActionDelete},
		ResourcePolicy:       {ActionView, ActionCreate, ActionUpdate, ActionDelete},
		ResourceSchedule:     {ActionView, ActionCreate, ActionUpdate, ActionDelete},
		ResourceQueryResult:  {ActionView, ActionDelete},
		ResourceYaraSource:   {ActionView, ActionCreate, ActionUpdate, ActionDelete},
		ResourceYaraScan:     {ActionView, ActionCreate},
		ResourceInventory:    {ActionView, ActionCreate, ActionUpdate, ActionDelete},
		ResourceUser:         {ActionView, ActionCreate, ActionUpdate, ActionDelete},
		ResourceUserGroup:    {ActionView, ActionCreate, ActionUpdate, ActionDelete},
		ResourceRoleBinding:  {ActionView, ActionCreate, ActionDelete},
		ResourceSetting:      {ActionView, ActionUpdate},
		ResourceAlertRule:    {ActionView, ActionCreate, ActionUpdate, ActionDelete},
		ResourceAlertTarget:  {ActionView, ActionCreate, ActionUpdate, ActionDelete, ActionExecute},
	},
	RoleOperator: {
		ResourceMachine:      {ActionView, ActionUpdate, ActionExecute, ActionDelete},
		ResourceMachineGroup: {ActionView, ActionCreate, ActionUpdate, ActionDelete},
		ResourcePolicy:       {ActionView, ActionCreate, ActionUpdate, ActionDelete},
		ResourceSchedule:     {ActionView, ActionCreate, ActionUpdate, ActionDelete},
		ResourceQueryResult:  {ActionView, ActionDelete},
		ResourceYaraSource:   {ActionView, ActionCreate, ActionUpdate, ActionDelete},
		ResourceYaraScan:     {ActionView, ActionCreate},
		ResourceInventory:    {ActionView, ActionCreate, ActionUpdate, ActionDelete},
		ResourceSetting:      {ActionView},
		ResourceAlertRule:    {ActionView, ActionCreate, ActionUpdate, ActionDelete},
		ResourceAlertTarget:  {ActionView, ActionCreate, ActionUpdate, ActionDelete, ActionExecute},
	},
	RoleAnalyst: {
		ResourceMachine:      {ActionView, ActionExecute},
		ResourceMachineGroup: {ActionView},
		ResourcePolicy:       {ActionView},
		ResourceSchedule:     {ActionView},
		ResourceQueryResult:  {ActionView},
		ResourceYaraSource:   {ActionView},
		ResourceYaraScan:     {ActionView, ActionCreate},
		ResourceInventory:    {ActionView},
		ResourceAlertRule:    {ActionView},
		ResourceAlertTarget:  {ActionView},
	},
	RoleViewer: {
		ResourceMachine:      {ActionView},
		ResourceMachineGroup: {ActionView},
		ResourcePolicy:       {ActionView},
		ResourceSchedule:     {ActionView},
		ResourceQueryResult:  {ActionView},
		ResourceYaraSource:   {ActionView},
		ResourceYaraScan:     {ActionView},
		ResourceInventory:    {ActionView},
		ResourceAlertRule:    {ActionView},
		ResourceAlertTarget:  {ActionView},
	},
}

var roleDescriptions = map[string]string{
	RoleAdmin:    "Full control, including users, role bindings, user groups, and settings.",
	RoleOperator: "Manage machines, groups, policies, schedules, YARA sources, scans, and inventory.",
	RoleAnalyst:  "Read everything; run live queries and YARA scans.",
	RoleViewer:   "Read-only across all fleet & detection data.",
}

// orderedRoles is the stable display order of the built-in roles.
var orderedRoles = []string{RoleAdmin, RoleOperator, RoleAnalyst, RoleViewer}

// IsBuiltinRole reports whether name is one of the four built-in roles.
func IsBuiltinRole(name string) bool {
	_, ok := roleMatrix[name]
	return ok
}

// NewEnforcer builds a Casbin enforcer backed by the postgres sqlx adapter,
// wrapping the existing *sql.DB
func NewEnforcer(db *sql.DB, rbacModel string) (*casbin.Enforcer, error) {
	m, err := casbinmodel.NewModelFromString(rbacModel)
	if err != nil {
		return nil, fmt.Errorf("parse rbac model: %w", err)
	}

	sqlxDB := sqlx.NewDb(db, "postgres")
	adapter := sqlxadapter.NewAdapterFromOptions(&sqlxadapter.AdapterOptions{DB: sqlxDB})

	enforcer, err := casbin.NewEnforcer(m, adapter)
	if err != nil {
		return nil, fmt.Errorf("create casbin enforcer: %w", err)
	}
	return enforcer, nil
}

func subjectFor(subjectType, subjectUUID string) string {
	return fmt.Sprintf("%s:%s", subjectType, subjectUUID)
}

func roleSubject(role string) string {
	return "role:" + role
}

// SeedBuiltinRoles writes the code-defined role permission matrix as `p` rows
// (domain "*") into the enforcer. Caller is responsible for persistence.
func (c *Core) SeedBuiltinRoles(enforcer *casbin.Enforcer) error {
	for _, role := range orderedRoles {
		resources := roleMatrix[role]
		// stable ordering for deterministic output (tests rely on this)
		resourceNames := make([]string, 0, len(resources))
		for resource := range resources {
			resourceNames = append(resourceNames, resource)
		}
		sort.Strings(resourceNames)
		for _, resource := range resourceNames {
			for _, action := range resources[resource] {
				if _, err := enforcer.AddPolicy(roleSubject(role), globalDomain, resource, action); err != nil {
					return fmt.Errorf("seed role %s policy %s:%s: %w", role, resource, action, err)
				}
			}
		}
	}
	return nil
}

// SyncPolicies rebuilds the shared casbin_rule store from the code-defined role
// matrix plus the role_bindings table
func (c *Core) SyncPolicies(ctx context.Context) error {
	if c.enforcer == nil {
		return errors.New("casbin enforcer not configured")
	}

	// Build everything in memory first, then persist once.
	c.enforcer.EnableAutoSave(false)
	defer c.enforcer.EnableAutoSave(true)

	c.enforcer.ClearPolicy()

	if err := c.SeedBuiltinRoles(c.enforcer); err != nil {
		return err
	}

	bindings, err := c.store.ListAllRoleBindings(ctx)
	if err != nil {
		return fmt.Errorf("list role bindings: %w", err)
	}
	for _, b := range bindings {
		domain := globalDomain
		if b.ScopeGroupUuid.Valid {
			domain = b.ScopeGroupUuid.UUID.String()
		}
		subject := subjectFor(b.SubjectType, b.SubjectUuid.String())
		if _, err := c.enforcer.AddGroupingPolicy(subject, roleSubject(b.Role), domain); err != nil {
			return fmt.Errorf("add binding grouping policy: %w", err)
		}
	}

	if err := c.enforcer.SavePolicy(); err != nil {
		return fmt.Errorf("save policy: %w", err)
	}
	return nil
}

// Can reports whether the user (directly or via its user groups) is permitted to
// perform action on resource. With no groupUUIDs the check is global (domain
// "*"); otherwise it passes if any of the supplied machine-group domains grants
// it (global bindings always satisfy via the matcher's "*" branch).
func (c *Core) Can(ctx context.Context, userUUID, resource, action string, groupUUIDs ...string) (bool, error) {
	if c.enforcer == nil {
		return false, errors.New("casbin enforcer not configured")
	}

	subjects := []string{subjectFor(SubjectUser, userUUID)}

	uid, err := uuid.Parse(userUUID)
	if err != nil {
		return false, fmt.Errorf("parse user uuid: %w", err)
	}
	user, err := c.store.GetUserByUUID(ctx, uid)
	if err != nil {
		return false, fmt.Errorf("get user: %w", err)
	}
	groups, err := c.store.ListUserGroupsForUser(ctx, user.ID)
	if err != nil {
		return false, fmt.Errorf("list user groups: %w", err)
	}
	for _, g := range groups {
		subjects = append(subjects, subjectFor(SubjectUserGroup, g.Uuid.String()))
	}

	domains := []string{globalDomain}
	if len(groupUUIDs) > 0 {
		domains = groupUUIDs
	}

	for _, subject := range subjects {
		for _, domain := range domains {
			allowed, err := c.enforcer.Enforce(subject, domain, resource, action)
			if err != nil {
				return false, fmt.Errorf("enforce: %w", err)
			}
			if allowed {
				return true, nil
			}
		}
	}
	return false, nil
}

// Roles returns the four built-in roles and their permission matrix
func (c *Core) Roles() []models.RoleDefinition {
	out := make([]models.RoleDefinition, 0, len(orderedRoles))
	for _, role := range orderedRoles {
		perms := make(map[string][]string, len(roleMatrix[role]))
		for resource, actions := range roleMatrix[role] {
			cp := make([]string, len(actions))
			copy(cp, actions)
			perms[resource] = cp
		}
		out = append(out, models.RoleDefinition{
			Name:        role,
			Description: roleDescriptions[role],
			Permissions: perms,
		})
	}
	return out
}

// PermissionCatalog returns the static resource × action catalog.
func (c *Core) PermissionCatalog() models.Catalog {
	entries := make([]models.PermissionCatalogEntry, len(permissionCatalog))
	copy(entries, permissionCatalog)
	return models.Catalog{Resources: entries}
}

// BindRole binds a subject (user or user group) to a built-in role, optionally
// scoped to a machine group. Dual-writes the role_bindings table and the
// enforcer, then persists. Idempotent.
func (c *Core) BindRole(ctx context.Context, subjectType, subjectUUID, role string, scopeGroupUUID *string) (models.RoleBinding, error) {
	if !IsBuiltinRole(role) {
		return models.RoleBinding{}, fmt.Errorf("%w: %s", ErrInvalidRole, role)
	}

	userID, groupID, canonicalUUID, err := c.resolveBindingSubject(ctx, subjectType, subjectUUID)
	if err != nil {
		return models.RoleBinding{}, err
	}
	scopeGroupID, domain, err := c.resolveScopeGroup(ctx, scopeGroupUUID)
	if err != nil {
		return models.RoleBinding{}, err
	}
	binding, err := c.findOrCreateRoleBinding(ctx, userID, groupID, role, scopeGroupID)
	if err != nil {
		return models.RoleBinding{}, err
	}

	if _, err := c.enforcer.AddGroupingPolicy(subjectFor(subjectType, canonicalUUID), roleSubject(role), domain); err != nil {
		return models.RoleBinding{}, fmt.Errorf("add grouping policy: %w", err)
	}
	if err := c.enforcer.SavePolicy(); err != nil {
		return models.RoleBinding{}, fmt.Errorf("save policy: %w", err)
	}

	return toModelRoleBinding(binding, scopeGroupUUID), nil
}

// resolveBindingSubject validates a subject (user or user group) by UUID and
// returns its id column plus the canonical UUID used for the casbin subject.
func (c *Core) resolveBindingSubject(ctx context.Context, subjectType, subjectUUID string) (userID, groupID sql.NullInt64, canonicalUUID string, err error) {
	id, err := uuid.Parse(subjectUUID)
	if err != nil {
		return userID, groupID, "", fmt.Errorf("parse subject uuid: %w", err)
	}
	switch subjectType {
	case SubjectUser:
		user, err := c.store.GetUserByUUID(ctx, id)
		if err != nil {
			return userID, groupID, "", fmt.Errorf("get user: %w", err)
		}
		return sql.NullInt64{Int64: user.ID, Valid: true}, groupID, user.Uuid.String(), nil
	case SubjectUserGroup:
		group, err := c.store.GetUserGroupByUUID(ctx, id)
		if err != nil {
			return userID, groupID, "", fmt.Errorf("get user group: %w", err)
		}
		return userID, sql.NullInt64{Int64: group.ID, Valid: true}, group.Uuid.String(), nil
	default:
		return userID, groupID, "", fmt.Errorf("invalid subject type: %s", subjectType)
	}
}

// resolveScopeGroup maps an optional machine-group UUID to its id column and the
// casbin domain, defaulting to a global (unscoped) binding when nil/empty.
func (c *Core) resolveScopeGroup(ctx context.Context, scopeGroupUUID *string) (sql.NullInt64, string, error) {
	if scopeGroupUUID == nil || *scopeGroupUUID == "" {
		return sql.NullInt64{}, globalDomain, nil
	}
	sgid, err := uuid.Parse(*scopeGroupUUID)
	if err != nil {
		return sql.NullInt64{}, "", fmt.Errorf("parse scope group uuid: %w", err)
	}
	group, err := c.store.GetGroupByUUID(ctx, sgid)
	if err != nil {
		return sql.NullInt64{}, "", fmt.Errorf("get scope group: %w", err)
	}
	return sql.NullInt64{Int64: group.ID, Valid: true}, group.Uuid.String(), nil
}

// findOrCreateRoleBinding returns the existing binding matching the tuple, or
// creates it. Idempotency relies on FindRoleBinding's IS NOT DISTINCT FROM match
// (the unique constraints are NULLS DISTINCT, so they don't dedupe global bindings).
func (c *Core) findOrCreateRoleBinding(ctx context.Context, userID, groupID sql.NullInt64, role string, scopeGroupID sql.NullInt64) (repo.RoleBinding, error) {
	existing, err := c.store.FindRoleBinding(ctx, repo.FindRoleBindingParams{
		UserID:       userID,
		UserGroupID:  groupID,
		Role:         role,
		ScopeGroupID: scopeGroupID,
	})
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return repo.RoleBinding{}, fmt.Errorf("find role binding: %w", err)
	}
	binding, err := c.store.CreateRoleBinding(ctx, repo.CreateRoleBindingParams{
		UserID:       userID,
		UserGroupID:  groupID,
		Role:         role,
		ScopeGroupID: scopeGroupID,
	})
	if err != nil {
		return repo.RoleBinding{}, fmt.Errorf("create role binding: %w", err)
	}
	return binding, nil
}

func toModelRoleBinding(b repo.RoleBinding, scopeGroupUUID *string) models.RoleBinding {
	out := models.RoleBinding{
		UUID:      b.Uuid.String(),
		Role:      b.Role,
		CreatedAt: b.CreatedAt,
	}
	if scopeGroupUUID != nil {
		out.ScopeGroupUUID = *scopeGroupUUID
	}
	return out
}

// UnbindRole removes a role binding by uuid and the matching enforcer grouping
// policy, then persists.
func (c *Core) UnbindRole(ctx context.Context, bindingUUID string) error {
	bid, err := uuid.Parse(bindingUUID)
	if err != nil {
		return fmt.Errorf("parse binding uuid: %w", err)
	}

	binding, err := c.store.GetRoleBindingByUUID(ctx, bid)
	if err != nil {
		return fmt.Errorf("get role binding: %w", err)
	}

	subject, err := c.subjectForBinding(ctx, binding)
	if err != nil {
		return err
	}

	domain := globalDomain
	if binding.ScopeGroupID.Valid {
		group, err := c.store.GetGroupByID(ctx, binding.ScopeGroupID.Int64)
		if err != nil {
			return fmt.Errorf("get scope group: %w", err)
		}
		domain = group.Uuid.String()
	}

	rows, err := c.store.DeleteRoleBindingByUUID(ctx, bid)
	if err != nil {
		return fmt.Errorf("delete role binding: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	if _, err := c.enforcer.RemoveGroupingPolicy(subject, roleSubject(binding.Role), domain); err != nil {
		return fmt.Errorf("remove grouping policy: %w", err)
	}
	if err := c.enforcer.SavePolicy(); err != nil {
		return fmt.Errorf("save policy: %w", err)
	}
	return nil
}

func (c *Core) subjectForBinding(ctx context.Context, binding repo.RoleBinding) (string, error) {
	switch {
	case binding.UserID.Valid:
		user, err := c.store.GetUserByID(ctx, binding.UserID.Int64)
		if err != nil {
			return "", fmt.Errorf("get binding user: %w", err)
		}
		return subjectFor(SubjectUser, user.Uuid.String()), nil
	case binding.UserGroupID.Valid:
		group, err := c.store.GetUserGroupByID(ctx, binding.UserGroupID.Int64)
		if err != nil {
			return "", fmt.Errorf("get binding user group: %w", err)
		}
		return subjectFor(SubjectUserGroup, group.Uuid.String()), nil
	default:
		return "", errors.New("role binding has no subject")
	}
}

// ListBindingsForSubject returns the role bindings attached to a subject.
func (c *Core) ListBindingsForSubject(ctx context.Context, subjectType, subjectUUID string) ([]models.RoleBinding, error) {
	sid, err := uuid.Parse(subjectUUID)
	if err != nil {
		return nil, fmt.Errorf("parse subject uuid: %w", err)
	}

	switch subjectType {
	case SubjectUser:
		user, err := c.store.GetUserByUUID(ctx, sid)
		if err != nil {
			return nil, fmt.Errorf("get user: %w", err)
		}
		rows, err := c.store.ListRoleBindingsForUser(ctx, sql.NullInt64{Int64: user.ID, Valid: true})
		if err != nil {
			return nil, fmt.Errorf("list user bindings: %w", err)
		}
		out := make([]models.RoleBinding, 0, len(rows))
		for _, r := range rows {
			out = append(out, models.RoleBinding{
				UUID:           r.Uuid.String(),
				Role:           r.Role,
				ScopeGroupUUID: nullUUIDString(r.ScopeGroupUuid),
				ScopeGroupName: r.ScopeGroupName.String,
				CreatedAt:      r.CreatedAt,
			})
		}
		return out, nil
	case SubjectUserGroup:
		group, err := c.store.GetUserGroupByUUID(ctx, sid)
		if err != nil {
			return nil, fmt.Errorf("get user group: %w", err)
		}
		rows, err := c.store.ListRoleBindingsForUserGroup(ctx, sql.NullInt64{Int64: group.ID, Valid: true})
		if err != nil {
			return nil, fmt.Errorf("list user group bindings: %w", err)
		}
		out := make([]models.RoleBinding, 0, len(rows))
		for _, r := range rows {
			out = append(out, models.RoleBinding{
				UUID:           r.Uuid.String(),
				Role:           r.Role,
				ScopeGroupUUID: nullUUIDString(r.ScopeGroupUuid),
				ScopeGroupName: r.ScopeGroupName.String,
				CreatedAt:      r.CreatedAt,
			})
		}
		return out, nil
	default:
		return nil, fmt.Errorf("invalid subject type: %s", subjectType)
	}
}

func nullUUIDString(u uuid.NullUUID) string {
	if u.Valid {
		return u.UUID.String()
	}
	return ""
}

// EffectivePermissions computes the global roles and merged permission map for a
// user (direct bindings + bindings inherited via user-group membership), used by
// the frontend for UI gating.
func (c *Core) EffectivePermissions(ctx context.Context, userUUID string) (models.EffectivePermissions, error) {
	uid, err := uuid.Parse(userUUID)
	if err != nil {
		return models.EffectivePermissions{}, fmt.Errorf("parse user uuid: %w", err)
	}
	user, err := c.store.GetUserByUUID(ctx, uid)
	if err != nil {
		return models.EffectivePermissions{}, fmt.Errorf("get user: %w", err)
	}

	roleSet := map[string]struct{}{}
	directRoles, err := c.store.ListGlobalRolesForUser(ctx, sql.NullInt64{Int64: user.ID, Valid: true})
	if err != nil {
		return models.EffectivePermissions{}, fmt.Errorf("list direct roles: %w", err)
	}
	for _, r := range directRoles {
		roleSet[r] = struct{}{}
	}
	groupRoles, err := c.store.ListGlobalRolesForUserGroups(ctx, user.ID)
	if err != nil {
		return models.EffectivePermissions{}, fmt.Errorf("list group roles: %w", err)
	}
	for _, r := range groupRoles {
		roleSet[r] = struct{}{}
	}

	roles := make([]string, 0, len(roleSet))
	for _, r := range orderedRoles {
		if _, ok := roleSet[r]; ok {
			roles = append(roles, r)
		}
	}

	// Merge the permission matrices for the held roles.
	merged := map[string]map[string]struct{}{}
	for r := range roleSet {
		for resource, actions := range roleMatrix[r] {
			if merged[resource] == nil {
				merged[resource] = map[string]struct{}{}
			}
			for _, a := range actions {
				merged[resource][a] = struct{}{}
			}
		}
	}

	perms := make(map[string][]string, len(merged))
	for resource, actionSet := range merged {
		// preserve catalog action order
		var ordered []string
		for _, entry := range permissionCatalog {
			if entry.Resource != resource {
				continue
			}
			for _, a := range entry.Actions {
				if _, ok := actionSet[a]; ok {
					ordered = append(ordered, a)
				}
			}
		}
		perms[resource] = ordered
	}

	return models.EffectivePermissions{
		User: models.SessionUser{
			UUID:      user.Uuid.String(),
			Username:  user.Username,
			Name:      user.Name,
			Email:     user.Email,
			LoginType: user.LoginType,
		},
		Roles:       roles,
		Permissions: perms,
	}, nil
}
