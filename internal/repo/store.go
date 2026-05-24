package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type Store interface {
	Querier

	CreatePolicyTx(ctx context.Context, params CreatePolicyTxParams) (Policy, error)
	CreateScheduleTx(ctx context.Context, params CreateScheduleTxParams) (Schedule, error)
	InsertStatusLogsTx(ctx context.Context, params InsertStatusLogsTxParams) error
	ListGroupsForSchedules(ctx context.Context, scheduleUUIDs []uuid.UUID) (map[uuid.UUID][]Group, error)
	ListSchedulesByNames(ctx context.Context, names []string) ([]Schedule, error)
	ReplaceNodeGroupsTx(ctx context.Context, params ReplaceNodeGroupsTxParams) error
	UpdatePolicyTx(ctx context.Context, params UpdatePolicyTxParams) (Policy, error)
	UpdateQueryTx(ctx context.Context, params UpdateQueryTxParams) (Query, error)
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

type CreateScheduleTxParams struct {
	Schedule   CreateScheduleParams
	GroupUUIDs []uuid.UUID
}

type UpdateScheduleTxParams struct {
	Schedule   UpdateScheduleByUUIDParams
	GroupUUIDs []uuid.UUID
}

type UpdateQueryTxParams struct {
	Query UpdateQueryByUUIDParams
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

func (s *PostgresStore) ListSchedulesByNames(ctx context.Context, names []string) ([]Schedule, error) {
	if len(names) == 0 {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, uuid, query_id, name, interval_seconds, platform, version, shard, denylist, removed,
       snapshot, enabled, is_system, sql_version, retention_days, created_at, updated_at
FROM schedules
WHERE name = ANY($1)
`, pq.Array(names))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Schedule, 0, len(names))
	for rows.Next() {
		var sched Schedule
		if err := rows.Scan(
			&sched.ID,
			&sched.Uuid,
			&sched.QueryID,
			&sched.Name,
			&sched.IntervalSeconds,
			&sched.Platform,
			&sched.Version,
			&sched.Shard,
			&sched.Denylist,
			&sched.Removed,
			&sched.Snapshot,
			&sched.Enabled,
			&sched.IsSystem,
			&sched.SqlVersion,
			&sched.RetentionDays,
			&sched.CreatedAt,
			&sched.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, sched)
	}
	return out, rows.Err()
}

func (s *PostgresStore) ListGroupsForSchedules(ctx context.Context, scheduleUUIDs []uuid.UUID) (map[uuid.UUID][]Group, error) {
	out := make(map[uuid.UUID][]Group, len(scheduleUUIDs))
	if len(scheduleUUIDs) == 0 {
		return out, nil
	}

	ids := make([]string, 0, len(scheduleUUIDs))
	for _, id := range scheduleUUIDs {
		ids = append(ids, id.String())
		out[id] = nil
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT schedules.uuid, groups.id, groups.uuid, groups.name, groups.description, groups.created_at, groups.updated_at
FROM groups
JOIN schedule_groups ON schedule_groups.group_id = groups.id
JOIN schedules ON schedules.id = schedule_groups.schedule_id
WHERE schedules.uuid = ANY($1::uuid[])
ORDER BY schedules.uuid, groups.name
`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var scheduleUUID uuid.UUID
		var group Group
		if err := rows.Scan(
			&scheduleUUID,
			&group.ID,
			&group.Uuid,
			&group.Name,
			&group.Description,
			&group.CreatedAt,
			&group.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out[scheduleUUID] = append(out[scheduleUUID], group)
	}
	return out, rows.Err()
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

	if existing.QueryID != params.Schedule.QueryID || existing.Snapshot != params.Schedule.Snapshot {
		if _, err := q.BumpScheduleVersion(ctx, existing.ID); err != nil {
			return Schedule{}, fmt.Errorf("bump schedule version: %w", err)
		}
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

func (s *PostgresStore) UpdateQueryTx(ctx context.Context, params UpdateQueryTxParams) (Query, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Query{}, fmt.Errorf("begin update query transaction: %w", err)
	}
	defer tx.Rollback()

	q := s.Queries.WithTx(tx)

	existing, err := q.GetQueryByUUID(ctx, params.Query.Uuid)
	if err != nil {
		return Query{}, fmt.Errorf("get existing query: %w", err)
	}

	updated, err := q.UpdateQueryByUUID(ctx, params.Query)
	if err != nil {
		return Query{}, fmt.Errorf("update query: %w", err)
	}

	if existing.Sql != updated.Sql {
		schedules, err := q.ListSchedulesForQuery(ctx, updated.ID)
		if err != nil {
			return Query{}, fmt.Errorf("list schedules for query: %w", err)
		}
		for _, sched := range schedules {
			if _, err := q.BumpScheduleVersion(ctx, sched.ID); err != nil {
				return Query{}, fmt.Errorf("bump schedule version for %s: %w", sched.Uuid, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return Query{}, fmt.Errorf("commit update query transaction: %w", err)
	}

	return updated, nil
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
