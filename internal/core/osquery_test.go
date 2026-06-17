package core

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/cvhariharan/checkpost/internal/repo"
	"github.com/google/uuid"
)

var discardLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type fakePolicyStore struct {
	repo.Store

	node                      repo.Node
	policies                  []repo.Policy
	listPoliciesErr           error
	policy                    repo.Policy
	getPolicyErr              error
	upserted                  []repo.UpsertPolicyMembershipParams
	lastPolicyCheckID         int64
	touchedNodeKeys           []uuid.UUID
	machineQueries            []repo.MachineQueryResult
	pendingMachineQueries     []repo.MachineQueryResult
	createdMachineQueries     []repo.CreateMachineQueryResultParams
	dispatchedMachineQueryIDs []int64
	completedMachineQueries   []repo.CompleteMachineQueryResultParams
	completedMachineResults   []repo.MachineQueryResult
	listMachineQueriesErr     error
	pendingMachineQueriesErr  error
	completeMachineQueryErr   error
	createMachineQueryErr     error
	dispatchMachineQueriesErr error
	deleteMachineQueryErr     error
	deletedMachineQueries     []repo.DeleteMachineQueryResultByNodeAndUUIDParams
	getNodeByUUIDErr          error
	touchNodeErr              error
}

func (s *fakePolicyStore) GetNodeByKey(ctx context.Context, nodeKey uuid.UUID) (repo.Node, error) {
	return s.node, nil
}

func (s *fakePolicyStore) GetNodeByUUID(ctx context.Context, id uuid.UUID) (repo.Node, error) {
	if s.getNodeByUUIDErr != nil {
		return repo.Node{}, s.getNodeByUUIDErr
	}
	return s.node, nil
}

func (s *fakePolicyStore) TouchNode(ctx context.Context, nodeKey uuid.UUID) error {
	if s.touchNodeErr != nil {
		return s.touchNodeErr
	}
	s.touchedNodeKeys = append(s.touchedNodeKeys, nodeKey)
	return nil
}

func (s *fakePolicyStore) ListEnabledPoliciesForNode(ctx context.Context, arg repo.ListEnabledPoliciesForNodeParams) ([]repo.Policy, error) {
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

func (s *fakePolicyStore) ListGroupsForNode(ctx context.Context, nodeUUID uuid.UUID) ([]repo.Group, error) {
	return nil, nil
}

func (s *fakePolicyStore) ListGroupsForPolicy(ctx context.Context, policyUUID uuid.UUID) ([]repo.Group, error) {
	return nil, nil
}

func (s *fakePolicyStore) UpsertPolicyMembership(ctx context.Context, arg repo.UpsertPolicyMembershipParams) error {
	s.upserted = append(s.upserted, arg)
	return nil
}

func (s *fakePolicyStore) UpdateNodeLastPolicyCheckAt(ctx context.Context, id int64) error {
	s.lastPolicyCheckID = id
	return nil
}

func (s *fakePolicyStore) CreateMachineQueryResult(ctx context.Context, arg repo.CreateMachineQueryResultParams) (repo.MachineQueryResult, error) {
	if s.createMachineQueryErr != nil {
		return repo.MachineQueryResult{}, s.createMachineQueryErr
	}

	now := time.Now().UTC()
	result := repo.MachineQueryResult{
		ID:        int64(len(s.machineQueries) + len(s.createdMachineQueries) + 1),
		Uuid:      arg.Uuid,
		NodeID:    arg.NodeID,
		Query:     arg.Query,
		Status:    "pending",
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.createdMachineQueries = append(s.createdMachineQueries, arg)
	s.machineQueries = append([]repo.MachineQueryResult{result}, s.machineQueries...)
	return result, nil
}

func (s *fakePolicyStore) ListMachineQueryResultsByNodeUUID(ctx context.Context, arg repo.ListMachineQueryResultsByNodeUUIDParams) ([]repo.ListMachineQueryResultsByNodeUUIDRow, error) {
	if s.listMachineQueriesErr != nil {
		return nil, s.listMachineQueriesErr
	}
	offset := int(arg.OffsetCount)
	if offset > len(s.machineQueries) {
		return nil, nil
	}
	limit := offset + int(arg.LimitCount)
	if limit > len(s.machineQueries) {
		limit = len(s.machineQueries)
	}
	out := make([]repo.ListMachineQueryResultsByNodeUUIDRow, 0, limit-offset)
	for _, row := range s.machineQueries[offset:limit] {
		out = append(out, repo.ListMachineQueryResultsByNodeUUIDRow{
			ID:           row.ID,
			Uuid:         row.Uuid,
			NodeID:       row.NodeID,
			Query:        row.Query,
			Status:       row.Status,
			RowCount:     row.RowCount,
			Error:        row.Error,
			DispatchedAt: row.DispatchedAt,
			CompletedAt:  row.CompletedAt,
			CreatedAt:    row.CreatedAt,
			UpdatedAt:    row.UpdatedAt,
			TotalCount:   int64(len(s.machineQueries)),
		})
	}
	return out, nil
}

func (s *fakePolicyStore) ListPendingMachineQueryResults(ctx context.Context, nodeID int64) ([]repo.MachineQueryResult, error) {
	if s.pendingMachineQueriesErr != nil {
		return nil, s.pendingMachineQueriesErr
	}
	out := make([]repo.MachineQueryResult, len(s.pendingMachineQueries))
	copy(out, s.pendingMachineQueries)
	return out, nil
}

func (s *fakePolicyStore) MarkMachineQueryResultsDispatched(ctx context.Context, ids []int64) error {
	if s.dispatchMachineQueriesErr != nil {
		return s.dispatchMachineQueriesErr
	}
	s.dispatchedMachineQueryIDs = append(s.dispatchedMachineQueryIDs, ids...)
	return nil
}

func (s *fakePolicyStore) DeleteMachineQueryResultByNodeAndUUID(ctx context.Context, arg repo.DeleteMachineQueryResultByNodeAndUUIDParams) (int64, error) {
	if s.deleteMachineQueryErr != nil {
		return 0, s.deleteMachineQueryErr
	}
	if arg.NodeUuid != s.node.Uuid {
		return 0, nil
	}

	s.deletedMachineQueries = append(s.deletedMachineQueries, arg)

	remaining := s.machineQueries[:0]
	deleted := int64(0)
	for _, row := range s.machineQueries {
		if row.Uuid == arg.QueryUuid && row.NodeID == s.node.ID {
			deleted++
			continue
		}
		remaining = append(remaining, row)
	}
	s.machineQueries = remaining
	return deleted, nil
}

func (s *fakePolicyStore) CompleteMachineQueryResult(ctx context.Context, arg repo.CompleteMachineQueryResultParams) (repo.MachineQueryResult, error) {
	if s.completeMachineQueryErr != nil {
		return repo.MachineQueryResult{}, s.completeMachineQueryErr
	}

	s.completedMachineQueries = append(s.completedMachineQueries, arg)
	for _, existing := range append(s.machineQueries, s.pendingMachineQueries...) {
		if existing.Uuid != arg.Uuid {
			continue
		}
		now := time.Now().UTC()
		status := "complete"
		if arg.Error != "" {
			status = "error"
		}
		completed := existing
		completed.Status = status
		completed.RowCount = arg.RowCount
		completed.Error = arg.Error
		completed.CompletedAt = sql.NullTime{Time: now, Valid: true}
		completed.UpdatedAt = now
		s.completedMachineResults = append(s.completedMachineResults, completed)
		return completed, nil
	}

	return repo.MachineQueryResult{}, sql.ErrNoRows
}

func TestReadDistributedQueriesMergesDuePolicyQueries(t *testing.T) {
	nodeKey := uuid.New()
	nodeUUID := uuid.New()
	policyUUID := uuid.New()
	adhocUUID := uuid.New()
	store := &fakePolicyStore{
		node: repo.Node{ID: 7, Uuid: nodeUUID, NodeKey: nodeKey, Platform: "linux"},
		policies: []repo.Policy{{
			ID:    99,
			Uuid:  policyUUID,
			Query: "SELECT 1;",
		}},
		pendingMachineQueries: []repo.MachineQueryResult{{
			ID:        123,
			Uuid:      adhocUUID,
			NodeID:    7,
			Query:     "SELECT * FROM processes;",
			Status:    "pending",
			CreatedAt: time.Now().UTC(),
		}},
	}
	core := &Core{
		store:                store,
		policyUpdateInterval: time.Hour,
	}

	queries, err := core.ReadDistributedQueries(context.Background(), models.NodeKeyRequest{NodeKey: nodeKey.String()})
	if err != nil {
		t.Fatalf("read distributed queries: %v", err)
	}

	if got := queries[adhocUUID.String()]; got != "SELECT * FROM processes;" {
		t.Fatalf("expected ad-hoc query to remain, got %q", got)
	}
	if len(store.dispatchedMachineQueryIDs) != 1 || store.dispatchedMachineQueryIDs[0] != 123 {
		t.Fatalf("expected pending ad-hoc query to be marked dispatched, got %+v", store.dispatchedMachineQueryIDs)
	}
	policyName := policyQueryPrefix + policyUUID.String()
	if got := queries[policyName]; got != "SELECT 1;" {
		t.Fatalf("expected policy query %s, got %q", policyName, got)
	}
}

func TestReadDistributedQueriesPreservesAdHocOnPolicyError(t *testing.T) {
	nodeKey := uuid.New()
	adhocUUID := uuid.New()
	store := &fakePolicyStore{
		node:            repo.Node{ID: 7, Uuid: uuid.New(), NodeKey: nodeKey, Platform: "linux"},
		listPoliciesErr: fmt.Errorf("transient db failure"),
		pendingMachineQueries: []repo.MachineQueryResult{{
			ID:     123,
			Uuid:   adhocUUID,
			NodeID: 7,
			Query:  "SELECT 1",
			Status: "pending",
		}},
	}
	core := &Core{
		store:                store,
		policyUpdateInterval: time.Hour,
	}

	_, err := core.ReadDistributedQueries(context.Background(), models.NodeKeyRequest{NodeKey: nodeKey.String()})
	if err == nil {
		t.Fatalf("expected policy lookup error")
	}
	if len(store.dispatchedMachineQueryIDs) != 0 {
		t.Fatalf("expected ad-hoc query to remain undispatched, got %+v", store.dispatchedMachineQueryIDs)
	}
}

func TestReadDistributedQueriesReturnsNoPoliciesSentinel(t *testing.T) {
	nodeKey := uuid.New()
	store := &fakePolicyStore{
		node: repo.Node{ID: 7, Uuid: uuid.New(), NodeKey: nodeKey, Platform: "windows"},
	}
	core := &Core{
		store:                store,
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

func TestReadDistributedQueriesRecordsHeartbeat(t *testing.T) {
	nodeKey := uuid.New()
	store := &fakePolicyStore{
		node: repo.Node{ID: 7, Uuid: uuid.New(), NodeKey: nodeKey, Platform: "linux"},
	}
	core := &Core{
		store:                store,
		policyUpdateInterval: time.Hour,
	}

	if _, err := core.ReadDistributedQueries(context.Background(), models.NodeKeyRequest{NodeKey: nodeKey.String()}); err != nil {
		t.Fatalf("read distributed queries: %v", err)
	}

	if len(store.touchedNodeKeys) != 1 || store.touchedNodeKeys[0] != nodeKey {
		t.Fatalf("expected distributed read to touch node %s, got %+v", nodeKey, store.touchedNodeKeys)
	}
}

func TestWriteDistributedQueryResultsRecordsPolicyAndAdHocResults(t *testing.T) {
	nodeKey := uuid.New()
	nodeUUID := uuid.New()
	policyUUID := uuid.New()
	adhocUUID := uuid.New()
	store := &fakePolicyStore{
		node:   repo.Node{ID: 7, Uuid: nodeUUID, NodeKey: nodeKey, Platform: "linux"},
		policy: repo.Policy{ID: 99, Uuid: policyUUID},
		machineQueries: []repo.MachineQueryResult{{
			ID:        123,
			Uuid:      adhocUUID,
			NodeID:    7,
			Query:     "SELECT 1",
			Status:    "pending",
			CreatedAt: time.Now().UTC(),
		}},
	}
	core := &Core{
		store:  store,
		logger: discardLogger,
	}

	err := core.WriteDistributedQueryResults(
		context.Background(),
		models.NodeKeyRequest{NodeKey: nodeKey.String()},
		map[string]interface{}{
			policyQueryPrefix + policyUUID.String(): []interface{}{map[string]interface{}{"one": "1"}},
			adhocUUID.String():                      []interface{}{},
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
	if store.lastPolicyCheckID != 7 {
		t.Fatalf("expected node policy check timestamp update for node 7, got %d", store.lastPolicyCheckID)
	}
	if len(store.completedMachineResults) != 1 {
		t.Fatalf("expected one completed ad-hoc result, got %d", len(store.completedMachineResults))
	}
	if got := store.completedMachineResults[0].Status; got != "complete" {
		t.Fatalf("expected ad-hoc result to complete, got %q", got)
	}
}

func TestWriteDistributedQueryResultsRecordsHeartbeat(t *testing.T) {
	nodeKey := uuid.New()
	store := &fakePolicyStore{
		node: repo.Node{ID: 7, Uuid: uuid.New(), NodeKey: nodeKey, Platform: "linux"},
	}
	core := &Core{
		store:  store,
		logger: discardLogger,
	}

	err := core.WriteDistributedQueryResults(
		context.Background(),
		models.NodeKeyRequest{NodeKey: nodeKey.String()},
		map[string]interface{}{},
		map[string]string{},
		map[string]string{},
	)
	if err != nil {
		t.Fatalf("write distributed query results: %v", err)
	}

	if len(store.touchedNodeKeys) != 1 || store.touchedNodeKeys[0] != nodeKey {
		t.Fatalf("expected distributed write to touch node %s, got %+v", nodeKey, store.touchedNodeKeys)
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
	core := &Core{store: store, logger: discardLogger}

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
		store:  store,
		logger: discardLogger,
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
	if store.lastPolicyCheckID != 0 {
		t.Fatalf("expected no policy check timestamp bump, got node %d", store.lastPolicyCheckID)
	}
}

func TestWriteDistributedQueryResultsPropagatesAdHocErrors(t *testing.T) {
	nodeKey := uuid.New()
	nodeUUID := uuid.New()
	adhocUUID := uuid.New()
	store := &fakePolicyStore{
		node: repo.Node{ID: 7, Uuid: nodeUUID, NodeKey: nodeKey, Platform: "linux"},
		machineQueries: []repo.MachineQueryResult{{
			ID:        123,
			Uuid:      adhocUUID,
			NodeID:    7,
			Query:     "SELECT * FROM nope",
			Status:    "pending",
			CreatedAt: time.Now().UTC(),
		}},
	}
	core := &Core{
		store:  store,
		logger: discardLogger,
	}

	err := core.WriteDistributedQueryResults(
		context.Background(),
		models.NodeKeyRequest{NodeKey: nodeKey.String()},
		map[string]interface{}{adhocUUID.String(): []interface{}{}},
		map[string]string{adhocUUID.String(): "1"},
		map[string]string{adhocUUID.String(): "no such table"},
	)
	if err != nil {
		t.Fatalf("write distributed query results: %v", err)
	}

	if len(store.completedMachineResults) != 1 {
		t.Fatalf("expected one completed ad-hoc result, got %d", len(store.completedMachineResults))
	}
	entry := store.completedMachineResults[0]
	if entry.Status != "error" {
		t.Fatalf("expected ad-hoc Status='error', got %q", entry.Status)
	}
	if entry.Error != "no such table" {
		t.Fatalf("expected ad-hoc Error='no such table', got %q", entry.Error)
	}

}

func TestWriteDistributedQueryResultsDrainsAdHocOnPolicyError(t *testing.T) {
	nodeKey := uuid.New()
	nodeUUID := uuid.New()
	policyUUID := uuid.New()
	adhocUUID := uuid.New()
	store := &fakePolicyStore{
		node:         repo.Node{ID: 7, Uuid: nodeUUID, NodeKey: nodeKey, Platform: "linux"},
		policy:       repo.Policy{ID: 99, Uuid: policyUUID},
		getPolicyErr: fmt.Errorf("transient db failure"),
		machineQueries: []repo.MachineQueryResult{{
			ID:        123,
			Uuid:      adhocUUID,
			NodeID:    7,
			Query:     "SELECT 1",
			Status:    "pending",
			CreatedAt: time.Now().UTC(),
		}},
	}
	core := &Core{
		store:  store,
		logger: discardLogger,
	}

	err := core.WriteDistributedQueryResults(
		context.Background(),
		models.NodeKeyRequest{NodeKey: nodeKey.String()},
		map[string]interface{}{
			policyQueryPrefix + policyUUID.String(): []interface{}{map[string]interface{}{"v": "1"}},
			adhocUUID.String():                      []interface{}{map[string]interface{}{"x": "y"}},
		},
		map[string]string{},
		map[string]string{},
	)
	if err == nil {
		t.Fatalf("expected policy DB error to surface")
	}

	if len(store.completedMachineResults) != 1 || store.completedMachineResults[0].Status != "complete" {
		t.Fatalf("expected ad-hoc result to be delivered despite policy failure")
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
		store:  store,
		logger: discardLogger,
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
