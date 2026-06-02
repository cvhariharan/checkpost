package core

import (
	"context"
	"testing"

	"github.com/cvhariharan/checkpost/internal/repo"
)

type fakeSyncStore struct {
	repo.Store

	byClaim         []repo.UserGroup
	upserted        []repo.UpsertUserGroupMemberOIDCParams
	staleDeleted    []repo.DeleteStaleOIDCMembersParams
	deletedAllForID int64
	deleteAllCalled bool
}

func (s *fakeSyncStore) ListUserGroupsByClaimValues(ctx context.Context, claimValues []string) ([]repo.UserGroup, error) {
	return s.byClaim, nil
}

func (s *fakeSyncStore) UpsertUserGroupMemberOIDC(ctx context.Context, arg repo.UpsertUserGroupMemberOIDCParams) error {
	s.upserted = append(s.upserted, arg)
	return nil
}

func (s *fakeSyncStore) DeleteStaleOIDCMembers(ctx context.Context, arg repo.DeleteStaleOIDCMembersParams) error {
	s.staleDeleted = append(s.staleDeleted, arg)
	return nil
}

func (s *fakeSyncStore) DeleteAllOIDCMembersForUser(ctx context.Context, userID int64) error {
	s.deleteAllCalled = true
	s.deletedAllForID = userID
	return nil
}

func TestSyncUserGroupsFromClaimsAddsAndPrunes(t *testing.T) {
	store := &fakeSyncStore{
		byClaim: []repo.UserGroup{{ID: 10}, {ID: 11}},
	}
	c := &Core{store: store}

	if err := c.SyncUserGroupsFromClaims(context.Background(), 42, []string{"Eng", "eng", "  SecOps  "}); err != nil {
		t.Fatalf("SyncUserGroupsFromClaims() error = %v", err)
	}

	if len(store.upserted) != 2 {
		t.Fatalf("upserted = %d, want 2", len(store.upserted))
	}
	for _, u := range store.upserted {
		if u.UserID != 42 {
			t.Fatalf("upsert UserID = %d, want 42", u.UserID)
		}
	}
	if len(store.staleDeleted) != 1 {
		t.Fatalf("staleDeleted calls = %d, want 1", len(store.staleDeleted))
	}
	if got := store.staleDeleted[0].KeepGroupIds; len(got) != 2 {
		t.Fatalf("KeepGroupIds = %v, want 2 ids", got)
	}
	if store.deleteAllCalled {
		t.Fatal("DeleteAllOIDCMembersForUser should not be called when groups match")
	}
}

func TestSyncUserGroupsFromClaimsNoMatchesRemovesAll(t *testing.T) {
	store := &fakeSyncStore{byClaim: nil}
	c := &Core{store: store}

	if err := c.SyncUserGroupsFromClaims(context.Background(), 7, []string{"unknown"}); err != nil {
		t.Fatalf("SyncUserGroupsFromClaims() error = %v", err)
	}

	if len(store.upserted) != 0 {
		t.Fatalf("upserted = %d, want 0", len(store.upserted))
	}
	if !store.deleteAllCalled || store.deletedAllForID != 7 {
		t.Fatalf("expected DeleteAllOIDCMembersForUser(7), got called=%v id=%d", store.deleteAllCalled, store.deletedAllForID)
	}
}
