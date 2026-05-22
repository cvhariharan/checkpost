package core

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/cvhariharan/watcher/internal/models"
	"github.com/cvhariharan/watcher/internal/repo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
)

func TestExecuteMachineQueryPersistsPendingResult(t *testing.T) {
	nodeUUID := uuid.New()
	store := &fakePolicyStore{
		node: repo.Node{ID: 7, Uuid: nodeUUID},
	}
	core := &Core{
		store: store,
	}

	result, err := core.ExecuteMachineQuery(context.Background(), models.MachineQueryRequest{
		NodeUUID: nodeUUID.String(),
		Query:    " SELECT 1 ",
	})
	if err != nil {
		t.Fatalf("execute machine query: %v", err)
	}

	if len(store.createdMachineQueries) != 1 {
		t.Fatalf("expected one persisted query, got %d", len(store.createdMachineQueries))
	}
	if store.createdMachineQueries[0].NodeID != 7 {
		t.Fatalf("expected query to target node 7, got %d", store.createdMachineQueries[0].NodeID)
	}
	if store.createdMachineQueries[0].Query != "SELECT 1" {
		t.Fatalf("expected trimmed query, got %q", store.createdMachineQueries[0].Query)
	}
	if result.Status != "pending" || result.Query != "SELECT 1" {
		t.Fatalf("unexpected pending result: %+v", result)
	}
}

func TestListMachineQueriesReturnsPersistedHistory(t *testing.T) {
	nodeUUID := uuid.New()
	queryUUID := uuid.New()
	olderQueryUUID := uuid.New()
	completedAt := time.Now().UTC()
	store := &fakePolicyStore{
		node: repo.Node{ID: 7, Uuid: nodeUUID},
		machineQueries: []repo.MachineQueryResult{
			{
				ID:          123,
				Uuid:        queryUUID,
				NodeID:      7,
				Query:       "SELECT name FROM processes LIMIT 1",
				Status:      "complete",
				Results:     pqtype.NullRawMessage{RawMessage: []byte(`[{"name":"watcher"}]`), Valid: true},
				CompletedAt: sql.NullTime{Time: completedAt, Valid: true},
				CreatedAt:   completedAt.Add(-time.Minute),
				UpdatedAt:   completedAt,
			},
			{
				ID:        124,
				Uuid:      olderQueryUUID,
				NodeID:    7,
				Query:     "SELECT 1",
				Status:    "pending",
				CreatedAt: completedAt.Add(-2 * time.Minute),
				UpdatedAt: completedAt.Add(-2 * time.Minute),
			},
		},
	}
	core := &Core{store: store}

	page, err := core.ListMachineQueries(context.Background(), models.NodeIdentity{ID: nodeUUID.String()}, models.PageRequest{
		Page:  0,
		Count: 1,
	})
	if err != nil {
		t.Fatalf("list machine queries: %v", err)
	}
	if page.TotalCount != 2 || page.PageCount != 2 {
		t.Fatalf("expected total/page count 2/2, got %d/%d", page.TotalCount, page.PageCount)
	}
	results := page.Items
	if len(results) != 1 {
		t.Fatalf("expected one query result, got %d", len(results))
	}
	if results[0].ID != queryUUID.String() || results[0].Status != "complete" {
		t.Fatalf("unexpected query result metadata: %+v", results[0])
	}
	rows, ok := results[0].Results.([]interface{})
	if !ok || len(rows) != 1 {
		t.Fatalf("expected decoded JSON rows, got %#v", results[0].Results)
	}
	row, ok := rows[0].(map[string]interface{})
	if !ok || row["name"] != "watcher" {
		t.Fatalf("expected decoded row, got %#v", rows[0])
	}
}

func TestDeleteMachineQueryRemovesMatchingResult(t *testing.T) {
	nodeUUID := uuid.New()
	queryUUID := uuid.New()
	otherQueryUUID := uuid.New()
	now := time.Now().UTC()
	store := &fakePolicyStore{
		node: repo.Node{ID: 7, Uuid: nodeUUID},
		machineQueries: []repo.MachineQueryResult{
			{ID: 1, Uuid: queryUUID, NodeID: 7, Query: "SELECT 1", Status: "complete", CreatedAt: now, UpdatedAt: now},
			{ID: 2, Uuid: otherQueryUUID, NodeID: 7, Query: "SELECT 2", Status: "complete", CreatedAt: now, UpdatedAt: now},
		},
	}
	core := &Core{store: store}

	err := core.DeleteMachineQuery(
		context.Background(),
		models.NodeIdentity{ID: nodeUUID.String()},
		models.ResourceID{UUID: queryUUID.String()},
	)
	if err != nil {
		t.Fatalf("delete machine query: %v", err)
	}
	if len(store.deletedMachineQueries) != 1 {
		t.Fatalf("expected one delete call, got %d", len(store.deletedMachineQueries))
	}
	if store.deletedMachineQueries[0].QueryUuid != queryUUID || store.deletedMachineQueries[0].NodeUuid != nodeUUID {
		t.Fatalf("unexpected delete arg: %+v", store.deletedMachineQueries[0])
	}
	if len(store.machineQueries) != 1 || store.machineQueries[0].Uuid != otherQueryUUID {
		t.Fatalf("expected only the unrelated query to remain, got %+v", store.machineQueries)
	}
}

func TestDeleteMachineQueryReturnsErrNoRowsWhenMissing(t *testing.T) {
	nodeUUID := uuid.New()
	store := &fakePolicyStore{
		node: repo.Node{ID: 7, Uuid: nodeUUID},
	}
	core := &Core{store: store}

	err := core.DeleteMachineQuery(
		context.Background(),
		models.NodeIdentity{ID: nodeUUID.String()},
		models.ResourceID{UUID: uuid.New().String()},
	)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestDeleteMachineQueryRejectsInvalidQueryUUID(t *testing.T) {
	nodeUUID := uuid.New()
	store := &fakePolicyStore{
		node: repo.Node{ID: 7, Uuid: nodeUUID},
	}
	core := &Core{store: store}

	err := core.DeleteMachineQuery(
		context.Background(),
		models.NodeIdentity{ID: nodeUUID.String()},
		models.ResourceID{UUID: "not-a-uuid"},
	)
	if err == nil {
		t.Fatal("expected error for invalid query uuid")
	}
	if len(store.deletedMachineQueries) != 0 {
		t.Fatalf("expected no store delete calls, got %d", len(store.deletedMachineQueries))
	}
}
