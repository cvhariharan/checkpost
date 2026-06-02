package core

import (
	"context"
	"testing"

	"github.com/casbin/casbin/v2"
	casbinmodel "github.com/casbin/casbin/v2/model"
	"github.com/cvhariharan/checkpost/internal/repo"
	"github.com/google/uuid"
)

// fakeRBACStore satisfies repo.Store; only the lookups used by Can are overridden.
type fakeRBACStore struct {
	repo.Store

	user   repo.User
	groups []repo.UserGroup
}

func (s *fakeRBACStore) GetUserByUUID(ctx context.Context, u uuid.UUID) (repo.User, error) {
	return s.user, nil
}

func (s *fakeRBACStore) ListUserGroupsForUser(ctx context.Context, userID int64) ([]repo.UserGroup, error) {
	return s.groups, nil
}

func newTestEnforcer(t *testing.T) *casbin.Enforcer {
	t.Helper()
	m, err := casbinmodel.NewModelFromString(rbacModel)
	if err != nil {
		t.Fatalf("parse model: %v", err)
	}
	e, err := casbin.NewEnforcer(m)
	if err != nil {
		t.Fatalf("new enforcer: %v", err)
	}
	e.EnableAutoSave(false) // no adapter in tests
	return e
}

func TestCanRoleMatrix(t *testing.T) {
	userUUID := uuid.New()

	cases := []struct {
		role     string
		resource string
		action   string
		want     bool
	}{
		// admin: full control
		{RoleAdmin, ResourceUser, ActionCreate, true},
		{RoleAdmin, ResourceMachine, ActionDelete, true},
		{RoleAdmin, ResourceRoleBinding, ActionCreate, true},
		{RoleAdmin, ResourceSetting, ActionUpdate, true},
		// operator: manages fleet/detection, no access management
		{RoleOperator, ResourceYaraSource, ActionUpdate, true},
		{RoleOperator, ResourceMachine, ActionExecute, true},
		{RoleOperator, ResourceUser, ActionView, false},
		{RoleOperator, ResourceSetting, ActionUpdate, false},
		{RoleOperator, ResourceSetting, ActionView, true},
		// analyst: read + run queries/scans, no config changes
		{RoleAnalyst, ResourceMachine, ActionExecute, true},
		{RoleAnalyst, ResourceYaraScan, ActionCreate, true},
		{RoleAnalyst, ResourceYaraSource, ActionUpdate, false},
		{RoleAnalyst, ResourcePolicy, ActionUpdate, false},
		{RoleAnalyst, ResourceUser, ActionView, false},
		// viewer: read-only
		{RoleViewer, ResourceMachine, ActionView, true},
		{RoleViewer, ResourceMachine, ActionExecute, false},
		{RoleViewer, ResourceYaraScan, ActionCreate, false},
	}

	for _, tc := range cases {
		t.Run(tc.role+"_"+tc.resource+"_"+tc.action, func(t *testing.T) {
			e := newTestEnforcer(t)
			c := &Core{enforcer: e, store: &fakeRBACStore{user: repo.User{ID: 1, Uuid: userUUID}}}
			if err := c.SeedBuiltinRoles(e); err != nil {
				t.Fatalf("seed roles: %v", err)
			}
			if _, err := e.AddGroupingPolicy(subjectFor(SubjectUser, userUUID.String()), roleSubject(tc.role), globalDomain); err != nil {
				t.Fatalf("add binding: %v", err)
			}

			got, err := c.Can(context.Background(), userUUID.String(), tc.resource, tc.action)
			if err != nil {
				t.Fatalf("Can() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("Can(%s,%s) with role %s = %v, want %v", tc.resource, tc.action, tc.role, got, tc.want)
			}
		})
	}
}

func TestCanViaUserGroup(t *testing.T) {
	userUUID := uuid.New()
	groupUUID := uuid.New()

	e := newTestEnforcer(t)
	c := &Core{
		enforcer: e,
		store: &fakeRBACStore{
			user:   repo.User{ID: 1, Uuid: userUUID},
			groups: []repo.UserGroup{{ID: 5, Uuid: groupUUID}},
		},
	}
	if err := c.SeedBuiltinRoles(e); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	// Bind the role to the user GROUP, not the user directly.
	if _, err := e.AddGroupingPolicy(subjectFor(SubjectUserGroup, groupUUID.String()), roleSubject(RoleOperator), globalDomain); err != nil {
		t.Fatalf("add binding: %v", err)
	}

	got, err := c.Can(context.Background(), userUUID.String(), ResourceYaraSource, ActionUpdate)
	if err != nil {
		t.Fatalf("Can() error = %v", err)
	}
	if !got {
		t.Fatal("Can() via user group = false, want true (operator grants yara_source:update)")
	}

	// The group does not grant user management.
	denied, err := c.Can(context.Background(), userUUID.String(), ResourceUser, ActionView)
	if err != nil {
		t.Fatalf("Can() error = %v", err)
	}
	if denied {
		t.Fatal("Can(user:view) via operator group = true, want false")
	}
}

func TestCanGlobalBindingSatisfiesScopedCheck(t *testing.T) {
	userUUID := uuid.New()
	groupUUID := uuid.New().String()

	e := newTestEnforcer(t)
	c := &Core{enforcer: e, store: &fakeRBACStore{user: repo.User{ID: 1, Uuid: userUUID}}}
	if err := c.SeedBuiltinRoles(e); err != nil {
		t.Fatalf("seed roles: %v", err)
	}
	// Global admin binding.
	if _, err := e.AddGroupingPolicy(subjectFor(SubjectUser, userUUID.String()), roleSubject(RoleAdmin), globalDomain); err != nil {
		t.Fatalf("add binding: %v", err)
	}

	// A group-scoped check should pass because the global binding satisfies any domain.
	got, err := c.Can(context.Background(), userUUID.String(), ResourceMachine, ActionUpdate, groupUUID)
	if err != nil {
		t.Fatalf("Can() error = %v", err)
	}
	if !got {
		t.Fatal("global admin binding did not satisfy a group-scoped check")
	}
}

func TestSeedBuiltinRolesMatrix(t *testing.T) {
	e := newTestEnforcer(t)
	c := &Core{enforcer: e}
	if err := c.SeedBuiltinRoles(e); err != nil {
		t.Fatalf("seed roles: %v", err)
	}

	// viewer holds exactly view on machine and nothing destructive.
	if ok, _ := e.Enforce(roleSubject(RoleViewer), globalDomain, ResourceMachine, ActionView); !ok {
		t.Fatal("viewer should allow machine:view")
	}
	if ok, _ := e.Enforce(roleSubject(RoleViewer), globalDomain, ResourceMachine, ActionDelete); ok {
		t.Fatal("viewer should not allow machine:delete")
	}
	// only admin touches user management.
	if ok, _ := e.Enforce(roleSubject(RoleOperator), globalDomain, ResourceUser, ActionView); ok {
		t.Fatal("operator should not allow user:view")
	}
	if ok, _ := e.Enforce(roleSubject(RoleAdmin), globalDomain, ResourceUser, ActionDelete); !ok {
		t.Fatal("admin should allow user:delete")
	}
}
