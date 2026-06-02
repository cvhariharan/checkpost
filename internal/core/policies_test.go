package core

import (
	"database/sql"
	"testing"
	"time"

	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/cvhariharan/checkpost/internal/repo"
	"github.com/google/uuid"
)

func TestValidatePolicyQuery(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{name: "select", query: "SELECT 1;", wantErr: false},
		{name: "with", query: "WITH checks AS (SELECT 1) SELECT * FROM checks", wantErr: false},
		{name: "empty", query: " ", wantErr: true},
		{name: "delete", query: "DELETE FROM users", wantErr: true},
		{name: "multiple statements", query: "SELECT 1; SELECT 2;", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePolicyQuery(tt.query)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestPolicyResultPasses(t *testing.T) {
	tests := []struct {
		name       string
		rows       interface{}
		wantPasses bool
		wantOK     bool
	}{
		{
			name:       "row with string 1 passes",
			rows:       []interface{}{map[string]interface{}{"one": "1"}},
			wantPasses: true,
			wantOK:     true,
		},
		{
			name:       "row with numeric 1 passes",
			rows:       []interface{}{map[string]interface{}{"one": float64(1)}},
			wantPasses: true,
			wantOK:     true,
		},
		{
			name:       "row with string 0 fails",
			rows:       []interface{}{map[string]interface{}{"result": "0"}},
			wantPasses: false,
			wantOK:     true,
		},
		{
			name:       "no rows fails",
			rows:       []interface{}{},
			wantPasses: false,
			wantOK:     true,
		},
		{
			name:       "nil rows fail",
			rows:       nil,
			wantPasses: false,
			wantOK:     true,
		},
		{
			name:       "non-binary value is malformed",
			rows:       []interface{}{map[string]interface{}{"result": "maybe"}},
			wantPasses: false,
			wantOK:     false,
		},
		{
			name:       "any 1 in multi-column row wins",
			rows:       []interface{}{map[string]interface{}{"a": "0", "b": "1"}},
			wantPasses: true,
			wantOK:     true,
		},
		{
			name:       "string row map fails on 0",
			rows:       []map[string]string{{"result": "0"}},
			wantPasses: false,
			wantOK:     true,
		},
		{
			name:       "boolean true value passes",
			rows:       []interface{}{map[string]interface{}{"result": true}},
			wantPasses: true,
			wantOK:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			passes, ok := policyResultPasses(tt.rows)
			if passes != tt.wantPasses || ok != tt.wantOK {
				t.Fatalf("policyResultPasses() = (%v, %v), want (%v, %v)", passes, ok, tt.wantPasses, tt.wantOK)
			}
		})
	}
}

func TestNodePolicyDue(t *testing.T) {
	core := &Core{policyUpdateInterval: time.Hour}
	now := time.Now()

	if !core.nodePolicyDue(models.Node{UUID: "node-a"}) {
		t.Fatalf("never-updated node should be due")
	}

	if core.nodePolicyDue(models.Node{UUID: "node-a", LastPolicyCheckAt: &now}) {
		t.Fatalf("fresh node should not be due")
	}

	old := now.Add(-2 * time.Hour)
	if !core.nodePolicyDue(models.Node{UUID: "node-a", LastPolicyCheckAt: &old}) {
		t.Fatalf("stale node should be due")
	}
}

func TestToModelPolicyPageCollapsesGroupRows(t *testing.T) {
	now := time.Now().UTC()
	policyAllID := uuid.New()
	policyGroupedID := uuid.New()
	groupAID := uuid.New()
	groupBID := uuid.New()

	rows := []repo.ListPoliciesWithCountsRow{
		{
			ID:           1,
			Uuid:         policyAllID,
			Name:         "All machines",
			Query:        "SELECT 1",
			Platform:     "all",
			Enabled:      true,
			CreatedAt:    now.Add(-time.Minute),
			UpdatedAt:    now,
			PassingCount: 1,
			UnknownCount: 2,
			TotalCount:   2,
		},
		{
			ID:               2,
			Uuid:             policyGroupedID,
			Name:             "Grouped",
			Query:            "SELECT 1",
			Platform:         "all",
			Enabled:          true,
			CreatedAt:        now,
			UpdatedAt:        now,
			PassingCount:     3,
			FailingCount:     4,
			UnknownCount:     5,
			TotalCount:       2,
			GroupID:          sql.NullInt64{Int64: 10, Valid: true},
			GroupUuid:        uuid.NullUUID{UUID: groupAID, Valid: true},
			GroupName:        sql.NullString{String: "Engineering", Valid: true},
			GroupDescription: sql.NullString{String: "eng nodes", Valid: true},
			GroupCreatedAt:   sql.NullTime{Time: now.Add(-2 * time.Hour), Valid: true},
			GroupUpdatedAt:   sql.NullTime{Time: now.Add(-time.Hour), Valid: true},
		},
		{
			ID:           2,
			Uuid:         policyGroupedID,
			Name:         "Grouped",
			Query:        "SELECT 1",
			Platform:     "all",
			Enabled:      true,
			CreatedAt:    now,
			UpdatedAt:    now,
			PassingCount: 3,
			FailingCount: 4,
			UnknownCount: 5,
			TotalCount:   2,
			GroupID:      sql.NullInt64{Int64: 11, Valid: true},
			GroupUuid:    uuid.NullUUID{UUID: groupBID, Valid: true},
			GroupName:    sql.NullString{String: "Security", Valid: true},
		},
	}

	policies, totalCount := toModelPolicyPage(rows)
	if totalCount != 2 {
		t.Fatalf("totalCount = %d, want 2", totalCount)
	}
	if len(policies) != 2 {
		t.Fatalf("len(policies) = %d, want 2", len(policies))
	}

	if !policies[0].TargetAllMachines {
		t.Fatalf("policy without groups should target all machines")
	}
	if len(policies[0].Groups) != 0 {
		t.Fatalf("policy without groups has %d groups, want 0", len(policies[0].Groups))
	}
	if policies[0].PassingCount != 1 || policies[0].UnknownCount != 2 {
		t.Fatalf("all-machine counts = (%d, %d), want (1, 2)", policies[0].PassingCount, policies[0].UnknownCount)
	}

	grouped := policies[1]
	if grouped.TargetAllMachines {
		t.Fatalf("policy with groups should not target all machines")
	}
	if len(grouped.Groups) != 2 {
		t.Fatalf("len(grouped.Groups) = %d, want 2", len(grouped.Groups))
	}
	if grouped.PassingCount != 3 || grouped.FailingCount != 4 || grouped.UnknownCount != 5 {
		t.Fatalf("grouped counts = (%d, %d, %d), want (3, 4, 5)", grouped.PassingCount, grouped.FailingCount, grouped.UnknownCount)
	}
	if grouped.Groups[0].UUID != groupAID.String() || grouped.Groups[0].Name != "Engineering" {
		t.Fatalf("first group = (%q, %q), want (%q, Engineering)", grouped.Groups[0].UUID, grouped.Groups[0].Name, groupAID.String())
	}
	if grouped.Groups[1].UUID != groupBID.String() || grouped.Groups[1].Name != "Security" {
		t.Fatalf("second group = (%q, %q), want (%q, Security)", grouped.Groups[1].UUID, grouped.Groups[1].Name, groupBID.String())
	}
}

func TestDeterministicPolicyJitter(t *testing.T) {
	interval := time.Hour
	first := deterministicPolicyJitter("node-a", interval)
	second := deterministicPolicyJitter("node-a", interval)

	if first != second {
		t.Fatalf("expected deterministic jitter, got %s and %s", first, second)
	}
	if first < 0 || first > interval/10 {
		t.Fatalf("expected jitter up to 10%% of interval, got %s", first)
	}
}
