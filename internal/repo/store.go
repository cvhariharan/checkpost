package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

type Store interface {
	Querier

	CreatePolicyTx(ctx context.Context, params CreatePolicyTxParams) (Policy, error)
	InsertResultBatchTx(ctx context.Context, params InsertResultBatchTxParams) error
	InsertStatusLogsTx(ctx context.Context, params InsertStatusLogsTxParams) error
	ReplaceNodeGroupsTx(ctx context.Context, params ReplaceNodeGroupsTxParams) error
	UpdatePolicyTx(ctx context.Context, params UpdatePolicyTxParams) (Policy, error)
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

type InsertResultBatchTxParams struct {
	Batch CreateResultBatchParams
	Rows  []InsertResultRowTxParams
}

type InsertResultRowTxParams struct {
	RowIndex int32
	Cells    []CreateResultCellTxParams
}

type CreateResultCellTxParams struct {
	ColumnName string
	ValueText  string
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

func (s *PostgresStore) InsertResultBatchTx(ctx context.Context, params InsertResultBatchTxParams) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin result batch transaction: %w", err)
	}
	defer tx.Rollback()

	q := s.Queries.WithTx(tx)

	batch, err := q.CreateResultBatch(ctx, params.Batch)
	if err != nil {
		return fmt.Errorf("create result batch: %w", err)
	}

	for _, rowParam := range params.Rows {
		row, err := q.CreateResultRow(ctx, CreateResultRowParams{
			BatchID:  batch.ID,
			RowIndex: rowParam.RowIndex,
		})
		if err != nil {
			return fmt.Errorf("create result row: %w", err)
		}

		for _, cellParam := range rowParam.Cells {
			if err := q.CreateResultCell(ctx, CreateResultCellParams{
				RowID:      row.ID,
				ColumnName: cellParam.ColumnName,
				ValueText:  cellParam.ValueText,
			}); err != nil {
				return fmt.Errorf("create result cell: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit result batch transaction: %w", err)
	}
	return nil
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
