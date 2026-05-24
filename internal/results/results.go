// Package results manages on-disk storage and querying of scheduled-query
// results. Results are written as Hive-partitioned Parquet files under a
// single root directory and queried through an embedded DuckDB engine.
//
// Layout:
//
//	{root}/
//	  query={schedule_uuid}/
//	    v={sql_version}/
//	      host={node_id}/
//	        chunk-{ulid}.parquet
package results

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

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

// ErrBackpressure is returned by Writer.Submit when both the per-partition
// channel and the global overflow buffer are full. Callers should propagate
// HTTP 503 to osquery so that it retries on the next interval.
var ErrBackpressure = errors.New("results: writer overflow buffer full")

// Row is a single result row received from osquery for one schedule.
type Row struct {
	NodeID       int64
	UnixTime     time.Time
	CalendarTime string
	Action       string // "added", "removed", or "snapshot"
	RowHash      []byte
	Columns      map[string]string
}

// PartitionKey identifies a single (schedule, version, host) partition.
type PartitionKey struct {
	ScheduleUUID uuid.UUID
	SQLVersion   int32
	NodeID       int64
}

// SchemaStore is the minimum surface from the metadata DB needed to resolve
// and persist column lists per (schedule_uuid, sql_version).
//
// MergeColumns atomically unions observed with the persisted list and
// returns the resulting authoritative column order (existing columns
// retain their position; new columns are appended in observed order).
// Doing the merge SQL-side is what makes concurrent partition writers
// safe — a read-modify-write split between Get and Upsert would let two
// workers each observe a different new column and clobber each other.
type SchemaStore interface {
	MergeColumns(ctx context.Context, scheduleUUID uuid.UUID, sqlVersion int32, observed []string) ([]string, error)
}

// partitionDir returns the directory holding chunk files for one partition.
func partitionDir(root string, key PartitionKey) string {
	return filepath.Join(
		root,
		fmt.Sprintf("query=%s", key.ScheduleUUID.String()),
		fmt.Sprintf("v=%d", key.SQLVersion),
		fmt.Sprintf("host=%d", key.NodeID),
	)
}

// versionDir returns the directory holding all hosts for one (schedule, version).
func versionDir(root string, scheduleUUID uuid.UUID, sqlVersion int32) string {
	return filepath.Join(
		root,
		fmt.Sprintf("query=%s", scheduleUUID.String()),
		fmt.Sprintf("v=%d", sqlVersion),
	)
}

// queryDir returns the directory holding all versions for one schedule.
func queryDir(root string, scheduleUUID uuid.UUID) string {
	return filepath.Join(root, fmt.Sprintf("query=%s", scheduleUUID.String()))
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
