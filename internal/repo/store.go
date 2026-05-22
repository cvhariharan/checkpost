package repo

import (
	"context"
	"database/sql"
	"fmt"
)

type Store interface {
	Querier

	InsertResultBatchTx(ctx context.Context, params InsertResultBatchTxParams) error
	InsertStatusLogsTx(ctx context.Context, params InsertStatusLogsTxParams) error
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
