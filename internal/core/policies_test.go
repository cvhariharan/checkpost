package core

import (
	"testing"
	"time"

	"github.com/cvhariharan/watcher/internal/models"
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
