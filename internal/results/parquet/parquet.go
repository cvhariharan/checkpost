// Package parquet implements the local result backend: scheduled-query results
// are written as Hive-partitioned Parquet files under a single root directory
// and queried through an embedded DuckDB engine. It satisfies both
// results.Sink (write/push) and results.Reader (frontend queries).
//
// Layout:
//
//	{root}/
//	  query={schedule_uuid}/
//	    v={sql_version}/
//	      host={node_id}/
//	        chunk-{ulid}.parquet
package parquet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/cvhariharan/checkpost/internal/repo"
	"github.com/cvhariharan/checkpost/internal/results"
	"github.com/google/uuid"
)

// Standard parquet column names that precede the auto-resolved query columns.
const (
	ColNodeID       = "node_id"
	ColUnixTime     = "unix_time"
	ColCalendarTime = "calendar_time"
	ColAction       = "action"
	ColRowHash      = "row_hash"
	ColIngestedAt   = "ingested_at"
)

// PartitionKey identifies a single (source, version, host) partition. Kind is
// passed to the schema store but is not part of the on-disk path.
type PartitionKey struct {
	SourceUUID uuid.UUID
	SQLVersion int32
	NodeID     int64
	Kind       string
}

// SchemaStore persists the column list per (source, sql_version). MergeColumns
// unions observed with the persisted list SQL-side (keeping existing order,
// appending new columns) so concurrent partition writers can't drop each
// other's columns.
type SchemaStore interface {
	MergeColumns(ctx context.Context, sourceUUID uuid.UUID, sqlVersion int32, observed []string, kind string) ([]string, error)
}

// Backend is the local Parquet+DuckDB result backend, satisfying results.Sink
// and results.Reader and owning the maintenance workers (compaction, retention,
// schema GC).
type Backend struct {
	writer  *Writer
	reader  *Reader
	workers *Workers
}

// New constructs the Parquet backend at root, with duckdbPath for the DuckDB
// catalog (empty = in-memory).
func New(root, duckdbPath string, store repo.Store, logger *slog.Logger, adhocRetentionDays int32) (*Backend, error) {
	writer, err := NewWriter(root, NewQueryStore(store), logger)
	if err != nil {
		return nil, err
	}
	reader, err := NewReader(root, duckdbPath)
	if err != nil {
		writer.Close()
		return nil, err
	}
	workers := NewWorkers(root, store, reader, logger, adhocRetentionDays)
	workers.Start()
	return &Backend{writer: writer, reader: reader, workers: workers}, nil
}

func (b *Backend) Name() string { return "parquet" }

func (b *Backend) Submit(ctx context.Context, batch results.Batch) error {
	return b.writer.Submit(batch.SourceUUID, batch.SQLVersion, batch.Kind, batch.Rows)
}

func (b *Backend) Delete(ctx context.Context, sourceUUID uuid.UUID) error {
	return b.writer.Delete(sourceUUID)
}

func (b *Backend) Read(ctx context.Context, sourceUUID uuid.UUID, sqlVersion int32, columns []string, opts results.ReadOptions) (results.Result, error) {
	return b.reader.Read(ctx, sourceUUID, sqlVersion, columns, opts)
}

// Close stops the workers before flushing and closing the writer and reader.
func (b *Backend) Close() error {
	b.workers.Close()
	werr := b.writer.Close()
	rerr := b.reader.Close()
	return errors.Join(werr, rerr)
}

// QueryStore is a repo.Querier-backed SchemaStore.
type QueryStore struct {
	q repo.Querier
}

func NewQueryStore(q repo.Querier) *QueryStore {
	return &QueryStore{q: q}
}

func (s *QueryStore) MergeColumns(ctx context.Context, sourceUUID uuid.UUID, sqlVersion int32, observed []string, kind string) ([]string, error) {
	raw, err := marshalColumns(observed)
	if err != nil {
		return nil, fmt.Errorf("encode columns: %w", err)
	}
	schema, err := s.q.UpsertQuerySchema(ctx, repo.UpsertQuerySchemaParams{
		SourceUuid: sourceUUID,
		SqlVersion: sqlVersion,
		Columns:    raw,
		Kind:       kind,
	})
	if err != nil {
		return nil, fmt.Errorf("merge query_schemas: %w", err)
	}
	cols, err := unmarshalColumns(schema.Columns)
	if err != nil {
		return nil, fmt.Errorf("decode columns: %w", err)
	}
	return cols, nil
}

// partitionDir returns the directory holding chunk files for one partition.
func partitionDir(root string, key PartitionKey) string {
	return filepath.Join(
		root,
		fmt.Sprintf("query=%s", key.SourceUUID.String()),
		fmt.Sprintf("v=%d", key.SQLVersion),
		fmt.Sprintf("host=%d", key.NodeID),
	)
}

func versionDir(root string, sourceUUID uuid.UUID, sqlVersion int32) string {
	return filepath.Join(
		root,
		fmt.Sprintf("query=%s", sourceUUID.String()),
		fmt.Sprintf("v=%d", sqlVersion),
	)
}

func queryDir(root string, sourceUUID uuid.UUID) string {
	return filepath.Join(root, fmt.Sprintf("query=%s", sourceUUID.String()))
}

// marshalColumns encodes a column list as the JSON representation stored in
// query_schemas.columns.
func marshalColumns(cols []string) (json.RawMessage, error) {
	if cols == nil {
		cols = []string{}
	}
	return json.Marshal(cols)
}

// unmarshalColumns decodes a query_schemas.columns blob into an ordered slice.
func unmarshalColumns(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var cols []string
	if err := json.Unmarshal(raw, &cols); err != nil {
		return nil, err
	}
	return cols, nil
}
