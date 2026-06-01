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
	CreateYaraScanTx(ctx context.Context, params CreateYaraScanTxParams) (YaraScan, error)
	InsertStatusLogsTx(ctx context.Context, params InsertStatusLogsTxParams) error
	ListNodesMatchingIdentity(ctx context.Context, term string) ([]MatchNodesByIdentityPatternRow, error)
	PatchGroupNodesTx(ctx context.Context, params PatchGroupNodesTxParams) error
	ReplaceNodeGroupsTx(ctx context.Context, params ReplaceNodeGroupsTxParams) error
	UpdateNodeInventoryTx(ctx context.Context, params UpdateNodeInventoryTxParams) (GetNodeInventoryByNodeUUIDRow, error)
	UpdatePolicyTx(ctx context.Context, params UpdatePolicyTxParams) (Policy, error)
	UpdateScheduleTx(ctx context.Context, params UpdateScheduleTxParams) (Schedule, error)
	CreateAlertRuleTx(ctx context.Context, params CreateAlertRuleTxParams) (AlertRule, error)
	UpdateAlertRuleTx(ctx context.Context, params UpdateAlertRuleTxParams) (AlertRule, error)
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

type UpdateNodeInventoryTxParams struct {
	NodeUUID           uuid.UUID
	OwnerUUID          *uuid.UUID
	InternalTrackingID string
	Notes              string
}

type CreateScheduleTxParams struct {
	Schedule   CreateScheduleParams
	GroupUUIDs []uuid.UUID
}

type UpdateScheduleTxParams struct {
	Schedule   UpdateScheduleByUUIDParams
	GroupUUIDs []uuid.UUID
}

type CreateYaraScanTxParams struct {
	GroupID  sql.NullInt64
	Paths    []string
	NodeIDs  []int64
	RuleURLs []string
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

func (s *PostgresStore) UpdateNodeInventoryTx(ctx context.Context, params UpdateNodeInventoryTxParams) (GetNodeInventoryByNodeUUIDRow, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return GetNodeInventoryByNodeUUIDRow{}, fmt.Errorf("begin update node inventory transaction: %w", err)
	}
	defer tx.Rollback()

	q := s.Queries.WithTx(tx)
	node, err := q.GetNodeByUUID(ctx, params.NodeUUID)
	if err != nil {
		return GetNodeInventoryByNodeUUIDRow{}, fmt.Errorf("get node: %w", err)
	}

	ownerID := sql.NullInt64{}
	if params.OwnerUUID != nil {
		owner, err := q.GetDeviceOwnerByUUID(ctx, *params.OwnerUUID)
		if err != nil {
			return GetNodeInventoryByNodeUUIDRow{}, fmt.Errorf("get owner: %w", err)
		}
		ownerID = sql.NullInt64{Int64: owner.ID, Valid: true}
	}

	if _, err := q.UpsertNodeInventory(ctx, UpsertNodeInventoryParams{
		NodeID:             node.ID,
		OwnerID:            ownerID,
		InternalTrackingID: params.InternalTrackingID,
		Notes:              params.Notes,
	}); err != nil {
		return GetNodeInventoryByNodeUUIDRow{}, fmt.Errorf("upsert node inventory: %w", err)
	}

	row, err := q.GetNodeInventoryByNodeUUID(ctx, params.NodeUUID)
	if err != nil {
		return GetNodeInventoryByNodeUUIDRow{}, fmt.Errorf("get node inventory: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return GetNodeInventoryByNodeUUIDRow{}, fmt.Errorf("commit update node inventory transaction: %w", err)
	}

	return row, nil
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

func (s *PostgresStore) CreateYaraScanTx(ctx context.Context, params CreateYaraScanTxParams) (YaraScan, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return YaraScan{}, fmt.Errorf("begin create yara scan transaction: %w", err)
	}
	defer tx.Rollback()

	q := s.Queries.WithTx(tx)
	scan, err := q.CreateYaraScan(ctx, CreateYaraScanParams{
		GroupID:     params.GroupID,
		Paths:       params.Paths,
		RuleUrls:    params.RuleURLs,
		TargetCount: int32(len(params.NodeIDs)),
	})
	if err != nil {
		return YaraScan{}, fmt.Errorf("create yara scan: %w", err)
	}

	for _, nodeID := range params.NodeIDs {
		if err := q.CreateYaraScanTarget(ctx, CreateYaraScanTargetParams{
			ScanID: scan.ID,
			NodeID: nodeID,
		}); err != nil {
			return YaraScan{}, fmt.Errorf("create yara scan target: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return YaraScan{}, fmt.Errorf("commit create yara scan transaction: %w", err)
	}

	return scan, nil
}

type CreateAlertRuleTxParams struct {
	Rule        CreateAlertRuleParams
	TargetUUIDs []uuid.UUID
}

type UpdateAlertRuleTxParams struct {
	Rule        UpdateAlertRuleByUUIDParams
	TargetUUIDs []uuid.UUID
}

func (s *PostgresStore) CreateAlertRuleTx(ctx context.Context, params CreateAlertRuleTxParams) (AlertRule, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AlertRule{}, fmt.Errorf("begin create alert rule transaction: %w", err)
	}
	defer tx.Rollback()

	q := s.Queries.WithTx(tx)
	rule, err := q.CreateAlertRule(ctx, params.Rule)
	if err != nil {
		return AlertRule{}, fmt.Errorf("create alert rule: %w", err)
	}
	if err := linkAlertTargetsTx(ctx, q, rule.ID, params.TargetUUIDs); err != nil {
		return AlertRule{}, err
	}
	if err := tx.Commit(); err != nil {
		return AlertRule{}, fmt.Errorf("commit create alert rule transaction: %w", err)
	}
	return rule, nil
}

func (s *PostgresStore) UpdateAlertRuleTx(ctx context.Context, params UpdateAlertRuleTxParams) (AlertRule, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AlertRule{}, fmt.Errorf("begin update alert rule transaction: %w", err)
	}
	defer tx.Rollback()

	q := s.Queries.WithTx(tx)
	rule, err := q.UpdateAlertRuleByUUID(ctx, params.Rule)
	if err != nil {
		return AlertRule{}, fmt.Errorf("update alert rule: %w", err)
	}
	if err := q.DeleteAlertRuleTargetsForRule(ctx, rule.ID); err != nil {
		return AlertRule{}, fmt.Errorf("clear alert rule targets: %w", err)
	}
	if err := linkAlertTargetsTx(ctx, q, rule.ID, params.TargetUUIDs); err != nil {
		return AlertRule{}, err
	}
	if err := tx.Commit(); err != nil {
		return AlertRule{}, fmt.Errorf("commit update alert rule transaction: %w", err)
	}
	return rule, nil
}

func linkAlertTargetsTx(ctx context.Context, q *Queries, ruleID int64, targetUUIDs []uuid.UUID) error {
	for _, tu := range targetUUIDs {
		target, err := q.GetAlertTargetByUUID(ctx, tu)
		if err != nil {
			return fmt.Errorf("get alert target %s: %w", tu, err)
		}
		if err := q.CreateAlertRuleTarget(ctx, CreateAlertRuleTargetParams{RuleID: ruleID, TargetID: target.ID}); err != nil {
			return fmt.Errorf("attach alert rule target: %w", err)
		}
	}
	return nil
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
