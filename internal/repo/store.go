package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type Store interface {
	Querier

	CreatePolicyTx(ctx context.Context, params CreatePolicyTxParams) (Policy, error)
	CreateScheduleTx(ctx context.Context, params CreateScheduleTxParams) (Schedule, error)
	InsertStatusLogsTx(ctx context.Context, params InsertStatusLogsTxParams) error
	ListNodesMatchingIdentity(ctx context.Context, term string) ([]MatchNodesByIdentityPatternRow, error)
	PatchGroupNodesTx(ctx context.Context, params PatchGroupNodesTxParams) error
	ReplaceNodeGroupsTx(ctx context.Context, params ReplaceNodeGroupsTxParams) error
	UpdatePolicyTx(ctx context.Context, params UpdatePolicyTxParams) (Policy, error)
	UpdateScheduleTx(ctx context.Context, params UpdateScheduleTxParams) (Schedule, error)
}

type PostgresStore struct {
	*Queries
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) Store {
	return &PostgresStore{
		Queries: New(db),
		db:      db,
	}
}

type InsertStatusLogsTxParams struct {
	Logs []CreateStatusLogParams
}

type CreatePolicyTxParams struct {
	Policy     CreatePolicyParams
	GroupUUIDs []uuid.UUID
}

type UpdatePolicyTxParams struct {
	Policy     UpdatePolicyByUUIDParams
	GroupUUIDs []uuid.UUID
}

type ReplaceNodeGroupsTxParams struct {
	NodeUUID   uuid.UUID
	GroupUUIDs []uuid.UUID
}

type PatchGroupNodesTxParams struct {
	GroupUUID       uuid.UUID
	AddNodeUUIDs    []uuid.UUID
	RemoveNodeUUIDs []uuid.UUID
}

type CreateScheduleTxParams struct {
	Schedule   CreateScheduleParams
	GroupUUIDs []uuid.UUID
}

type UpdateScheduleTxParams struct {
	Schedule   UpdateScheduleByUUIDParams
	GroupUUIDs []uuid.UUID
}

// ListNodesMatchingIdentity wraps the generated pattern lookup, applying
// the LIKE escape so user-supplied % and _ aren't treated as wildcards.
func (s *PostgresStore) ListNodesMatchingIdentity(ctx context.Context, term string) ([]MatchNodesByIdentityPatternRow, error) {
	return s.Queries.MatchNodesByIdentityPattern(ctx, MatchNodesByIdentityPatternParams{
		Pattern:  escapeLikePattern(term),
		MaxCount: 10000,
	})
}

func escapeLikePattern(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '%' || r == '_' || r == '\\' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (s *PostgresStore) InsertStatusLogsTx(ctx context.Context, params InsertStatusLogsTxParams) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin status logs transaction: %w", err)
	}
	defer tx.Rollback()

	q := s.Queries.WithTx(tx)
	for _, log := range params.Logs {
		if _, err := q.CreateStatusLog(ctx, log); err != nil {
			return fmt.Errorf("create status log: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit status logs transaction: %w", err)
	}
	return nil
}

func (s *PostgresStore) CreatePolicyTx(ctx context.Context, params CreatePolicyTxParams) (Policy, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Policy{}, fmt.Errorf("begin create policy transaction: %w", err)
	}
	defer tx.Rollback()

	q := s.Queries.WithTx(tx)
	groups, err := loadGroupsTx(ctx, q, params.GroupUUIDs)
	if err != nil {
		return Policy{}, err
	}

	policy, err := q.CreatePolicy(ctx, params.Policy)
	if err != nil {
		return Policy{}, fmt.Errorf("create policy: %w", err)
	}

	for _, group := range groups {
		if err := q.CreatePolicyGroup(ctx, CreatePolicyGroupParams{
			PolicyID: policy.ID,
			GroupID:  group.ID,
		}); err != nil {
			return Policy{}, fmt.Errorf("attach policy group: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Policy{}, fmt.Errorf("commit create policy transaction: %w", err)
	}

	return policy, nil
}

func (s *PostgresStore) UpdatePolicyTx(ctx context.Context, params UpdatePolicyTxParams) (Policy, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Policy{}, fmt.Errorf("begin update policy transaction: %w", err)
	}
	defer tx.Rollback()

	q := s.Queries.WithTx(tx)
	groups, err := loadGroupsTx(ctx, q, params.GroupUUIDs)
	if err != nil {
		return Policy{}, err
	}

	policy, err := q.UpdatePolicyByUUID(ctx, params.Policy)
	if err != nil {
		return Policy{}, fmt.Errorf("update policy: %w", err)
	}

	if err := q.DeletePolicyGroupsForPolicy(ctx, policy.Uuid); err != nil {
		return Policy{}, fmt.Errorf("clear policy groups: %w", err)
	}

	for _, group := range groups {
		if err := q.CreatePolicyGroup(ctx, CreatePolicyGroupParams{
			PolicyID: policy.ID,
			GroupID:  group.ID,
		}); err != nil {
			return Policy{}, fmt.Errorf("attach policy group: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Policy{}, fmt.Errorf("commit update policy transaction: %w", err)
	}

	return policy, nil
}

func (s *PostgresStore) ReplaceNodeGroupsTx(ctx context.Context, params ReplaceNodeGroupsTxParams) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin replace node groups transaction: %w", err)
	}
	defer tx.Rollback()

	q := s.Queries.WithTx(tx)
	node, err := q.GetNodeByUUID(ctx, params.NodeUUID)
	if err != nil {
		return fmt.Errorf("get node: %w", err)
	}

	groups, err := loadGroupsTx(ctx, q, params.GroupUUIDs)
	if err != nil {
		return err
	}

	if err := q.DeleteGroupMembershipsForNode(ctx, params.NodeUUID); err != nil {
		return fmt.Errorf("clear node groups: %w", err)
	}

	for _, group := range groups {
		if err := q.CreateGroupMembership(ctx, CreateGroupMembershipParams{
			GroupID: group.ID,
			NodeID:  node.ID,
		}); err != nil {
			return fmt.Errorf("attach node group: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit replace node groups transaction: %w", err)
	}

	return nil
}

func (s *PostgresStore) PatchGroupNodesTx(ctx context.Context, params PatchGroupNodesTxParams) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin patch group nodes transaction: %w", err)
	}
	defer tx.Rollback()

	q := s.Queries.WithTx(tx)
	group, err := q.GetGroupByUUID(ctx, params.GroupUUID)
	if err != nil {
		return fmt.Errorf("get group: %w", err)
	}

	nodes, err := loadNodesTx(ctx, q, params.AddNodeUUIDs)
	if err != nil {
		return err
	}

	for _, node := range nodes {
		if err := q.CreateGroupMembership(ctx, CreateGroupMembershipParams{
			GroupID: group.ID,
			NodeID:  node.ID,
		}); err != nil {
			return fmt.Errorf("attach group node: %w", err)
		}
	}

	for _, nodeUUID := range params.RemoveNodeUUIDs {
		if err := q.DeleteGroupMembershipForNode(ctx, DeleteGroupMembershipForNodeParams{
			GroupUuid: params.GroupUUID,
			NodeUuid:  nodeUUID,
		}); err != nil {
			return fmt.Errorf("detach group node: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit patch group nodes transaction: %w", err)
	}

	return nil
}

func (s *PostgresStore) CreateScheduleTx(ctx context.Context, params CreateScheduleTxParams) (Schedule, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Schedule{}, fmt.Errorf("begin create schedule transaction: %w", err)
	}
	defer tx.Rollback()

	q := s.Queries.WithTx(tx)
	groups, err := loadGroupsTx(ctx, q, params.GroupUUIDs)
	if err != nil {
		return Schedule{}, err
	}

	schedule, err := q.CreateSchedule(ctx, params.Schedule)
	if err != nil {
		return Schedule{}, fmt.Errorf("create schedule: %w", err)
	}

	for _, group := range groups {
		if err := q.CreateScheduleGroup(ctx, CreateScheduleGroupParams{
			ScheduleID: schedule.ID,
			GroupID:    group.ID,
		}); err != nil {
			return Schedule{}, fmt.Errorf("attach schedule group: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Schedule{}, fmt.Errorf("commit create schedule transaction: %w", err)
	}

	return schedule, nil
}

func (s *PostgresStore) UpdateScheduleTx(ctx context.Context, params UpdateScheduleTxParams) (Schedule, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Schedule{}, fmt.Errorf("begin update schedule transaction: %w", err)
	}
	defer tx.Rollback()

	q := s.Queries.WithTx(tx)
	groups, err := loadGroupsTx(ctx, q, params.GroupUUIDs)
	if err != nil {
		return Schedule{}, err
	}

	existing, err := q.GetScheduleByUUID(ctx, params.Schedule.Uuid)
	if err != nil {
		return Schedule{}, fmt.Errorf("get existing schedule: %w", err)
	}

	if existing.Sql != params.Schedule.Sql || existing.Snapshot != params.Schedule.Snapshot {
		if _, err := q.BumpScheduleVersion(ctx, existing.ID); err != nil {
			return Schedule{}, fmt.Errorf("bump schedule version: %w", err)
		}
	}

	// Treat retention_days=0 as "keep current"; the column is NOT NULL with a
	// CHECK between 1 and 365, so 0 is never a valid update value anyway.
	if params.Schedule.RetentionDays == 0 {
		params.Schedule.RetentionDays = existing.RetentionDays
	}

	schedule, err := q.UpdateScheduleByUUID(ctx, params.Schedule)
	if err != nil {
		return Schedule{}, fmt.Errorf("update schedule: %w", err)
	}

	if err := q.DeleteScheduleGroupsForSchedule(ctx, schedule.Uuid); err != nil {
		return Schedule{}, fmt.Errorf("clear schedule groups: %w", err)
	}

	for _, group := range groups {
		if err := q.CreateScheduleGroup(ctx, CreateScheduleGroupParams{
			ScheduleID: schedule.ID,
			GroupID:    group.ID,
		}); err != nil {
			return Schedule{}, fmt.Errorf("attach schedule group: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Schedule{}, fmt.Errorf("commit update schedule transaction: %w", err)
	}

	return schedule, nil
}

func loadGroupsTx(ctx context.Context, q *Queries, groupUUIDs []uuid.UUID) ([]Group, error) {
	groups := make([]Group, 0, len(groupUUIDs))
	for _, groupUUID := range groupUUIDs {
		group, err := q.GetGroupByUUID(ctx, groupUUID)
		if err != nil {
			return nil, fmt.Errorf("get group %s: %w", groupUUID, err)
		}
		groups = append(groups, group)
	}
	return groups, nil
}

func loadNodesTx(ctx context.Context, q *Queries, nodeUUIDs []uuid.UUID) ([]Node, error) {
	nodes := make([]Node, 0, len(nodeUUIDs))
	for _, nodeUUID := range nodeUUIDs {
		node, err := q.GetNodeByUUID(ctx, nodeUUID)
		if err != nil {
			return nil, fmt.Errorf("get node %s: %w", nodeUUID, err)
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}
