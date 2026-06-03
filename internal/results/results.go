// Package results defines the generic Sink/Reader interfaces and shared types
// for delivering scheduled-query results. Concrete backends live in subpackages
// (e.g. internal/results/parquet).
package results

import (
	"context"
	"errors"
	"time"

	"github.com/cvhariharan/checkpost/internal/resultquery"
	"github.com/google/uuid"
)

// ErrBackpressure is returned by Submit when a required backend's buffer is
// full. Callers should propagate HTTP 503 to osquery so that it retries on the
// next interval.
var ErrBackpressure = errors.New("results: backend overflow buffer full")

// Row is a single result row received from osquery for one schedule.
type Row struct {
	NodeID       int64
	UnixTime     time.Time
	CalendarTime string
	Action       string // "added", "removed", or "snapshot"
	RowHash      []byte
	Columns      map[string]string
}

// Batch is the rows osquery reported for one (schedule, sql_version) in a
// single ingest call, plus schedule metadata for external backends to tag.
type Batch struct {
	ScheduleUUID uuid.UUID
	SQLVersion   int32
	ScheduleName string
	Snapshot     bool
	Rows         []Row
}

// Sink is the generic write/push target for scheduled-query results.
type Sink interface {
	Name() string
	// Submit enqueues a batch. It returns ErrBackpressure when the backend's
	// buffer is full; best-effort backends never return ErrBackpressure.
	Submit(ctx context.Context, batch Batch) error
	DeleteSchedule(ctx context.Context, scheduleUUID uuid.UUID) error
	Close() error
}

// Reader is the frontend query surface. May be nil when no reader-capable
// backend is enabled (browsing disabled).
type Reader interface {
	Read(ctx context.Context, scheduleUUID uuid.UUID, sqlVersion int32, columns []string, opts ReadOptions) (Result, error)
	Close() error
}

// ReadOptions controls a single Reader.Read call.
type ReadOptions struct {
	NodeID   int64
	NodeIDs  []int64
	Snapshot bool
	Limit    int
	Offset   int
	Filters  []resultquery.Term
}

// Result is one page of read rows plus the total matching count.
type Result struct {
	Columns []string
	Rows    []ResultRow
	Total   int
}

// ResultRow is a single row returned to the frontend.
type ResultRow struct {
	NodeID   int64
	UnixTime time.Time
	Action   string
	Values   map[string]string
}
