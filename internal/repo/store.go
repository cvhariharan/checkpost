package repo

import (
	"context"
	"database/sql"
	"fmt"
)

type Store interface {
	Querier

	GetNodeByUUID(ctx context.Context, id string) (Node, error)

	InsertResultBatchTx(ctx context.Context, params InsertResultBatchTxParams) error
	InsertStatusLogsTx(ctx context.Context, params InsertStatusLogsTxParams) error
}

type PostgresStore struct {
	db *sql.DB
	*Queries
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{
		db:      db,
		Queries: New(db),
	}
}

func (s *PostgresStore) GetNodeByUUID(ctx context.Context, id string) (Node, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, uuid, node_key, host_identifier, hostname, platform, os_name, os_version, osquery_version, hardware_serial, enrolled_at, last_seen_at, created_at, updated_at, policy_updated_at
		FROM nodes
		WHERE uuid = $1
	`, id)

	var node Node
	if err := row.Scan(
		&node.ID,
		&node.Uuid,
		&node.NodeKey,
		&node.HostIdentifier,
		&node.Hostname,
		&node.Platform,
		&node.OsName,
		&node.OsVersion,
		&node.OsqueryVersion,
		&node.HardwareSerial,
		&node.EnrolledAt,
		&node.LastSeenAt,
		&node.CreatedAt,
		&node.UpdatedAt,
		&node.PolicyUpdatedAt,
	); err != nil {
		return Node{}, err
	}

	return node, nil
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
