package core

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/cvhariharan/watcher/internal/models"
	"github.com/cvhariharan/watcher/internal/repo"
	"github.com/google/uuid"
)

var discardLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type fakePolicyStore struct {
	repo.Store

	node              repo.Node
	policies          []repo.Policy
	listPoliciesErr   error
	policy            repo.Policy
	getPolicyErr      error
	upserted          []repo.UpsertPolicyMembershipParams
	policyTimestampID int64
}

func (s *fakePolicyStore) GetNodeByKey(ctx context.Context, nodeKey uuid.UUID) (repo.Node, error) {
	return s.node, nil
}

func (s *fakePolicyStore) ListEnabledPoliciesForPlatform(ctx context.Context, nodePlatform string) ([]repo.Policy, error) {
	if s.listPoliciesErr != nil {
		return nil, s.listPoliciesErr
	}
	return s.policies, nil
}

func (s *fakePolicyStore) GetPolicyByUUID(ctx context.Context, policyUUID uuid.UUID) (repo.Policy, error) {
	if s.getPolicyErr != nil {
		return repo.Policy{}, s.getPolicyErr
	}
	return s.policy, nil
}

func (s *fakePolicyStore) UpsertPolicyMembership(ctx context.Context, arg repo.UpsertPolicyMembershipParams) error {
	s.upserted = append(s.upserted, arg)
	return nil
}

func (s *fakePolicyStore) UpdateNodePolicyUpdatedAt(ctx context.Context, id int64) error {
	s.policyTimestampID = id
	return nil
}

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

	if core.nodePolicyDue(models.Node{UUID: "node-a", PolicyUpdatedAt: &now}) {
		t.Fatalf("fresh node should not be due")
	}

	old := now.Add(-2 * time.Hour)
	if !core.nodePolicyDue(models.Node{UUID: "node-a", PolicyUpdatedAt: &old}) {
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

func TestReadDistributedQueriesMergesDuePolicyQueries(t *testing.T) {
	nodeKey := uuid.New()
	nodeUUID := uuid.New()
	policyUUID := uuid.New()
	store := &fakePolicyStore{
		node: repo.Node{ID: 7, Uuid: nodeUUID, NodeKey: nodeKey, Platform: "linux"},
		policies: []repo.Policy{{
			ID:    99,
			Uuid:  policyUUID,
			Query: "SELECT 1;",
		}},
	}
	core := &Core{
		store:                store,
		adHocPending:         map[string][]models.MachineQueryResult{nodeKey.String(): {{ID: "adhoc", Query: "SELECT * FROM processes;"}}},
		adHocNodeKeyToID:     map[string]string{},
		policyUpdateInterval: time.Hour,
	}

	queries, err := core.ReadDistributedQueries(context.Background(), models.NodeKeyRequest{NodeKey: nodeKey.String()})
	if err != nil {
		t.Fatalf("read distributed queries: %v", err)
	}

	if got := queries["adhoc"]; got != "SELECT * FROM processes;" {
		t.Fatalf("expected ad-hoc query to remain, got %q", got)
	}
	policyName := policyQueryPrefix + policyUUID.String()
	if got := queries[policyName]; got != "SELECT 1;" {
		t.Fatalf("expected policy query %s, got %q", policyName, got)
	}
}

func TestReadDistributedQueriesPreservesAdHocOnPolicyError(t *testing.T) {
	nodeKey := uuid.New()
	store := &fakePolicyStore{
		node:            repo.Node{ID: 7, Uuid: uuid.New(), NodeKey: nodeKey, Platform: "linux"},
		listPoliciesErr: fmt.Errorf("transient db failure"),
	}
	core := &Core{
		store:                store,
		adHocPending:         map[string][]models.MachineQueryResult{nodeKey.String(): {{ID: "adhoc", Query: "SELECT 1"}}},
		adHocNodeKeyToID:     map[string]string{},
		policyUpdateInterval: time.Hour,
	}

	_, err := core.ReadDistributedQueries(context.Background(), models.NodeKeyRequest{NodeKey: nodeKey.String()})
	if err == nil {
		t.Fatalf("expected policy lookup error")
	}
	if len(core.adHocPending[nodeKey.String()]) != 1 {
		t.Fatalf("expected ad-hoc query to remain pending")
	}
}

func TestReadDistributedQueriesReturnsNoPoliciesSentinel(t *testing.T) {
	nodeKey := uuid.New()
	store := &fakePolicyStore{
		node: repo.Node{ID: 7, Uuid: uuid.New(), NodeKey: nodeKey, Platform: "windows"},
	}
	core := &Core{
		store:                store,
		adHocPending:         map[string][]models.MachineQueryResult{},
		adHocNodeKeyToID:     map[string]string{},
		policyUpdateInterval: time.Hour,
	}

	queries, err := core.ReadDistributedQueries(context.Background(), models.NodeKeyRequest{NodeKey: nodeKey.String()})
	if err != nil {
		t.Fatalf("read distributed queries: %v", err)
	}
	if got := queries[noPoliciesQueryID]; got != "SELECT 1" {
		t.Fatalf("expected no-policy sentinel, got %q", got)
	}
}

func TestWriteDistributedQueryResultsRecordsPolicyAndAdHocResults(t *testing.T) {
	nodeKey := uuid.New()
	nodeUUID := uuid.New()
	policyUUID := uuid.New()
	store := &fakePolicyStore{
		node:   repo.Node{ID: 7, Uuid: nodeUUID, NodeKey: nodeKey, Platform: "linux"},
		policy: repo.Policy{ID: 99, Uuid: policyUUID},
	}
	core := &Core{
		store:            store,
		logger:           discardLogger,
		adHocPending:     map[string][]models.MachineQueryResult{},
		adHocHistory:     map[string][]models.MachineQueryResult{nodeUUID.String(): {{ID: "adhoc", Query: "SELECT 1", Status: "pending"}}},
		adHocNodeKeyToID: map[string]string{},
		adHocWaiters:     map[string]chan models.MachineQueryResult{},
	}

	err := core.WriteDistributedQueryResults(
		context.Background(),
		models.NodeKeyRequest{NodeKey: nodeKey.String()},
		map[string]interface{}{
			policyQueryPrefix + policyUUID.String(): []interface{}{map[string]interface{}{"one": "1"}},
			"adhoc":                                 []interface{}{},
		},
		map[string]string{policyQueryPrefix + policyUUID.String(): "0"},
		map[string]string{},
	)
	if err != nil {
		t.Fatalf("write distributed query results: %v", err)
	}

	if len(store.upserted) != 1 {
		t.Fatalf("expected one policy upsert, got %d", len(store.upserted))
	}
	if !store.upserted[0].Passes.Valid || !store.upserted[0].Passes.Bool {
		t.Fatalf("expected passing policy result, got %+v", store.upserted[0].Passes)
	}
	if store.policyTimestampID != 7 {
		t.Fatalf("expected node policy timestamp update for node 7, got %d", store.policyTimestampID)
	}
	if got := core.adHocHistory[nodeUUID.String()][0].Status; got != "complete" {
		t.Fatalf("expected ad-hoc result to complete, got %q", got)
	}
}

func TestWriteDistributedQueryResultsRecordsPolicyFailuresAndErrors(t *testing.T) {
	nodeKey := uuid.New()
	nodeUUID := uuid.New()
	policyUUID := uuid.New()
	store := &fakePolicyStore{
		node:   repo.Node{ID: 7, Uuid: nodeUUID, NodeKey: nodeKey, Platform: "linux"},
		policy: repo.Policy{ID: 99, Uuid: policyUUID},
	}
	core := &Core{store: store, logger: discardLogger, adHocHistory: map[string][]models.MachineQueryResult{}, adHocNodeKeyToID: map[string]string{}, adHocWaiters: map[string]chan models.MachineQueryResult{}}

	err := core.WriteDistributedQueryResults(
		context.Background(),
		models.NodeKeyRequest{NodeKey: nodeKey.String()},
		map[string]interface{}{policyQueryPrefix + policyUUID.String(): []interface{}{}},
		map[string]string{policyQueryPrefix + policyUUID.String(): "0"},
		map[string]string{},
	)
	if err != nil {
		t.Fatalf("write failing policy result: %v", err)
	}
	if !store.upserted[0].Passes.Valid || store.upserted[0].Passes.Bool {
		t.Fatalf("expected failing policy result, got %+v", store.upserted[0].Passes)
	}

	err = core.WriteDistributedQueryResults(
		context.Background(),
		models.NodeKeyRequest{NodeKey: nodeKey.String()},
		map[string]interface{}{policyQueryPrefix + policyUUID.String(): []interface{}{}},
		map[string]string{policyQueryPrefix + policyUUID.String(): "1"},
		map[string]string{policyQueryPrefix + policyUUID.String(): "no such table"},
	)
	if err != nil {
		t.Fatalf("write errored policy result: %v", err)
	}
	last := store.upserted[len(store.upserted)-1]
	if last.Passes.Valid || last.LastError != "no such table" {
		t.Fatalf("expected unknown policy result with error, got passes=%+v error=%q", last.Passes, last.LastError)
	}
}

func TestWriteDistributedQueryResultsIgnoresPolicyKeysOnlyInStatuses(t *testing.T) {
	nodeKey := uuid.New()
	policyUUID := uuid.New()
	store := &fakePolicyStore{
		node:   repo.Node{ID: 7, Uuid: uuid.New(), NodeKey: nodeKey, Platform: "linux"},
		policy: repo.Policy{ID: 99, Uuid: policyUUID},
	}
	core := &Core{
		store:            store,
		logger:           discardLogger,
		adHocHistory:     map[string][]models.MachineQueryResult{},
		adHocNodeKeyToID: map[string]string{},
		adHocWaiters:     map[string]chan models.MachineQueryResult{},
	}

	err := core.WriteDistributedQueryResults(
		context.Background(),
		models.NodeKeyRequest{NodeKey: nodeKey.String()},
		map[string]interface{}{},
		map[string]string{policyQueryPrefix + policyUUID.String(): "1"},
		map[string]string{policyQueryPrefix + policyUUID.String(): "forged"},
	)
	if err != nil {
		t.Fatalf("write distributed query results: %v", err)
	}
	if len(store.upserted) != 0 {
		t.Fatalf("expected no upsert when policy queryID is missing from results, got %d", len(store.upserted))
	}
	if store.policyTimestampID != 0 {
		t.Fatalf("expected no policy timestamp bump, got node %d", store.policyTimestampID)
	}
}

func TestWriteDistributedQueryResultsPropagatesAdHocErrors(t *testing.T) {
	nodeKey := uuid.New()
	nodeUUID := uuid.New()
	store := &fakePolicyStore{
		node: repo.Node{ID: 7, Uuid: nodeUUID, NodeKey: nodeKey, Platform: "linux"},
	}
	waiter := make(chan models.MachineQueryResult, 1)
	core := &Core{
		store:            store,
		logger:           discardLogger,
		adHocPending:     map[string][]models.MachineQueryResult{},
		adHocHistory:     map[string][]models.MachineQueryResult{nodeUUID.String(): {{ID: "adhoc", Query: "SELECT * FROM nope", Status: "pending"}}},
		adHocNodeKeyToID: map[string]string{},
		adHocWaiters:     map[string]chan models.MachineQueryResult{"adhoc": waiter},
	}

	err := core.WriteDistributedQueryResults(
		context.Background(),
		models.NodeKeyRequest{NodeKey: nodeKey.String()},
		map[string]interface{}{"adhoc": []interface{}{}},
		map[string]string{"adhoc": "1"},
		map[string]string{"adhoc": "no such table"},
	)
	if err != nil {
		t.Fatalf("write distributed query results: %v", err)
	}

	entry := core.adHocHistory[nodeUUID.String()][0]
	if entry.Status != "error" {
		t.Fatalf("expected ad-hoc Status='error', got %q", entry.Status)
	}
	if entry.Error != "no such table" {
		t.Fatalf("expected ad-hoc Error='no such table', got %q", entry.Error)
	}

	select {
	case completed := <-waiter:
		if completed.Status != "error" || completed.Error != "no such table" {
			t.Fatalf("waiter got unexpected result: %+v", completed)
		}
	default:
		t.Fatalf("expected waiter to be signaled")
	}
}

func TestWriteDistributedQueryResultsDrainsAdHocOnPolicyError(t *testing.T) {
	nodeKey := uuid.New()
	nodeUUID := uuid.New()
	policyUUID := uuid.New()
	store := &fakePolicyStore{
		node:         repo.Node{ID: 7, Uuid: nodeUUID, NodeKey: nodeKey, Platform: "linux"},
		policy:       repo.Policy{ID: 99, Uuid: policyUUID},
		getPolicyErr: fmt.Errorf("transient db failure"),
	}
	waiter := make(chan models.MachineQueryResult, 1)
	core := &Core{
		store:            store,
		logger:           discardLogger,
		adHocPending:     map[string][]models.MachineQueryResult{},
		adHocHistory:     map[string][]models.MachineQueryResult{nodeUUID.String(): {{ID: "adhoc", Query: "SELECT 1", Status: "pending"}}},
		adHocNodeKeyToID: map[string]string{},
		adHocWaiters:     map[string]chan models.MachineQueryResult{"adhoc": waiter},
	}

	err := core.WriteDistributedQueryResults(
		context.Background(),
		models.NodeKeyRequest{NodeKey: nodeKey.String()},
		map[string]interface{}{
			policyQueryPrefix + policyUUID.String(): []interface{}{map[string]interface{}{"v": "1"}},
			"adhoc":                                 []interface{}{map[string]interface{}{"x": "y"}},
		},
		map[string]string{},
		map[string]string{},
	)
	if err == nil {
		t.Fatalf("expected policy DB error to surface")
	}

	if core.adHocHistory[nodeUUID.String()][0].Status != "complete" {
		t.Fatalf("expected ad-hoc result to be delivered despite policy failure")
	}
	select {
	case <-waiter:
	default:
		t.Fatalf("expected ad-hoc waiter to be signaled despite policy failure")
	}
}

func TestWriteDistributedQueryResultsIgnoresDeletedPolicy(t *testing.T) {
	nodeKey := uuid.New()
	policyUUID := uuid.New()
	store := &fakePolicyStore{
		node:         repo.Node{ID: 7, Uuid: uuid.New(), NodeKey: nodeKey, Platform: "linux"},
		getPolicyErr: sql.ErrNoRows,
	}
	core := &Core{
		store:            store,
		logger:           discardLogger,
		adHocHistory:     map[string][]models.MachineQueryResult{},
		adHocNodeKeyToID: map[string]string{},
		adHocWaiters:     map[string]chan models.MachineQueryResult{},
	}

	err := core.WriteDistributedQueryResults(
		context.Background(),
		models.NodeKeyRequest{NodeKey: nodeKey.String()},
		map[string]interface{}{policyQueryPrefix + policyUUID.String(): []interface{}{}},
		map[string]string{},
		map[string]string{},
	)
	if err != nil {
		t.Fatalf("deleted policy result should be ignored: %v", err)
	}
	if len(store.upserted) != 0 {
		t.Fatalf("expected no upsert for deleted policy, got %d", len(store.upserted))
	}
}
