// Package clickhouse implements a result Sink that inserts rows into a
// ClickHouse table. osquery's dynamic columns are stored in a Map(String,
// String) so the schema is stable across schedules.
package clickhouse

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"sync/atomic"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/cvhariharan/checkpost/internal/repo"
	"github.com/cvhariharan/checkpost/internal/results"
	"github.com/google/uuid"
)

const (
	queueSize     = 8192
	maxBatch      = 1000
	flushInterval = 5 * time.Second
)

// SchemaStore records observed columns into query_schemas so the reader can
// resolve a schedule's columns; this lets ClickHouse serve reads when Parquet
// (the usual recorder) is disabled. repo.Store satisfies it.
type SchemaStore interface {
	UpsertQuerySchema(ctx context.Context, arg repo.UpsertQuerySchemaParams) (repo.QuerySchema, error)
}

type schemaKey struct {
	scheduleUUID uuid.UUID
	sqlVersion   int32
	kind         string
}

type row struct {
	scheduleUUID uuid.UUID
	scheduleName string
	sqlVersion   int32
	kind         string
	nodeID       int64
	unixTime     time.Time
	calendarTime string
	action       string
	rowHash      string
	columns      map[string]string
}

// Sink buffers rows and inserts them in batches from a single goroutine.
type Sink struct {
	conn  driver.Conn
	table string
	in    chan row
	flush   chan chan struct{}
	done    chan struct{}
	logger  *slog.Logger
	dropped atomic.Uint64

	schema SchemaStore
	// seen tracks already-recorded columns per bucket; touched only by run(), so
	// it needs no synchronization.
	seen map[schemaKey]map[string]struct{}
}

// New connects to ClickHouse, ensures the results table exists, and starts the
// batch inserter. table defaults to "query_results"; ttlDays > 0 sets a TTL.
// schema records observed columns into query_schemas (nil disables recording).
func New(dsn, table string, ttlDays int, schema SchemaStore, logger *slog.Logger) (*Sink, error) {
	if table == "" {
		table = "query_results"
	}
	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse clickhouse dsn: %w", err)
	}
	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open clickhouse: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := conn.Ping(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping clickhouse: %w", err)
	}
	if err := ensureTable(ctx, conn, table, ttlDays); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ensure clickhouse table: %w", err)
	}

	s := &Sink{
		conn:   conn,
		table:  table,
		in:     make(chan row, queueSize),
		flush:  make(chan chan struct{}, 1),
		done:   make(chan struct{}),
		logger: logger.WithGroup("results.clickhouse"),
		schema: schema,
		seen:   make(map[schemaKey]map[string]struct{}),
	}
	go s.run()
	return s, nil
}

func (s *Sink) Name() string { return "clickhouse" }

func (s *Sink) Submit(ctx context.Context, batch results.Batch) error {
	for _, r := range batch.Rows {
		select {
		case s.in <- row{
			scheduleUUID: batch.SourceUUID,
			scheduleName: batch.SourceName,
			sqlVersion:   batch.SQLVersion,
			kind:         batch.Kind,
			nodeID:       r.NodeID,
			unixTime:     r.UnixTime.UTC(),
			calendarTime: r.CalendarTime,
			action:       r.Action,
			rowHash:      fmt.Sprintf("%x", r.RowHash),
			columns:      r.Columns,
		}:
		default:
			s.dropped.Add(1)
		}
	}
	return nil
}

func (s *Sink) Delete(ctx context.Context, sourceUUID uuid.UUID) error {
	return s.conn.Exec(ctx,
		fmt.Sprintf("ALTER TABLE %s DELETE WHERE schedule_uuid = {id:UUID}", s.table),
		clickhouse.Named("id", sourceUUID),
	)
}

// Flush inserts any buffered rows synchronously
func (s *Sink) Flush(ctx context.Context, sourceUUID uuid.UUID) error {
	ack := make(chan struct{})
	select {
	case s.flush <- ack:
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-ack:
		return nil
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Sink) Close() error {
	close(s.in)
	<-s.done
	return nil
}

func (s *Sink) run() {
	defer close(s.done)
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()
	buf := make([]row, 0, maxBatch)
	flush := func() {
		if len(buf) == 0 {
			return
		}
		s.recordSchema(buf)
		if err := s.insert(buf); err != nil {
			s.logger.Error("insert batch", "rows", len(buf), "error", err)
		}
		buf = buf[:0]
	}
	for {
		select {
		case r, ok := <-s.in:
			if !ok {
				flush()
				s.conn.Close()
				return
			}
			buf = append(buf, r)
			if len(buf) >= maxBatch {
				flush()
			}
		case ack := <-s.flush:
			// Drain everything already queued so the flush captures all rows
			// submitted before this request, then insert them.
		drain:
			for {
				select {
				case r, ok := <-s.in:
					if !ok {
						flush()
						close(ack)
						s.conn.Close()
						return
					}
					buf = append(buf, r)
				default:
					break drain
				}
			}
			flush()
			close(ack)
		case <-ticker.C:
			flush()
			if n := s.dropped.Swap(0); n > 0 {
				s.logger.Warn("dropped records on overflow", "count", n)
			}
		}
	}
}

func (s *Sink) insert(rows []row) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+s.table+
		" (schedule_uuid, sql_version, schedule_name, node_id, unix_time, calendar_time, action, row_hash, columns)")
	if err != nil {
		return err
	}
	for _, r := range rows {
		cols := r.columns
		if cols == nil {
			cols = map[string]string{}
		}
		if err := batch.Append(r.scheduleUUID, r.sqlVersion, r.scheduleName, r.nodeID,
			r.unixTime, r.calendarTime, r.action, r.rowHash, cols); err != nil {
			return err
		}
	}
	return batch.Send()
}

// recordSchema upserts columns in buf not yet seen for their (schedule,
// sql_version). The merge is cumulative and idempotent across instances; seen
// advances only on success, so failures retry on the next flush.
func (s *Sink) recordSchema(rows []row) {
	if s.schema == nil {
		return
	}
	fresh := make(map[schemaKey]map[string]struct{})
	for _, r := range rows {
		key := schemaKey{scheduleUUID: r.scheduleUUID, sqlVersion: r.sqlVersion, kind: r.kind}
		known := s.seen[key]
		for col := range r.columns {
			if col == "" {
				continue
			}
			if _, ok := known[col]; ok {
				continue
			}
			cols := fresh[key]
			if cols == nil {
				cols = make(map[string]struct{})
				fresh[key] = cols
			}
			cols[col] = struct{}{}
		}
	}
	if len(fresh) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for key, cols := range fresh {
		observed := make([]string, 0, len(cols))
		for col := range cols {
			observed = append(observed, col)
		}
		sort.Strings(observed)
		raw, err := json.Marshal(observed)
		if err != nil {
			s.logger.Warn("encode query schema columns", "error", err)
			continue
		}
		if _, err := s.schema.UpsertQuerySchema(ctx, repo.UpsertQuerySchemaParams{
			SourceUuid: key.scheduleUUID,
			SqlVersion: key.sqlVersion,
			Columns:    raw,
			Kind:       key.kind,
		}); err != nil {
			s.logger.Warn("record query schema", "source_uuid", key.scheduleUUID, "error", err)
			continue
		}
		seen := s.seen[key]
		if seen == nil {
			seen = make(map[string]struct{})
			s.seen[key] = seen
		}
		for col := range cols {
			seen[col] = struct{}{}
		}
	}
}

// ensureTable probes for the table first (needs no DDL) and creates it only
// when missing, so deployments where the user lacks DDL still work.
func ensureTable(ctx context.Context, conn driver.Conn, table string, ttlDays int) error {
	exists, err := tableExists(ctx, conn, table)
	if err != nil {
		return fmt.Errorf("check table exists: %w", err)
	}
	if exists {
		return nil
	}

	ttl := ""
	if ttlDays > 0 {
		ttl = fmt.Sprintf("\nTTL toDateTime(unix_time) + INTERVAL %d DAY", ttlDays)
	}
	ddl := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
	schedule_uuid UUID,
	sql_version   Int32,
	schedule_name String,
	node_id       Int64,
	unix_time     DateTime64(6, 'UTC'),
	calendar_time String,
	action        LowCardinality(String),
	row_hash      String,
	ingested_at   DateTime64(6, 'UTC') DEFAULT now64(6),
	columns       Map(String, String)
) ENGINE = MergeTree
PARTITION BY (schedule_uuid, toYYYYMM(unix_time))
ORDER BY (schedule_uuid, sql_version, node_id, unix_time)%s`, table, ttl)
	if err := conn.Exec(ctx, ddl); err != nil {
		return fmt.Errorf("create table %s (user may lack DDL permission; create the table out of band): %w", table, err)
	}
	return nil
}

// tableExists checks via EXISTS, which needs only read access. table may be
// qualified as db.name.
func tableExists(ctx context.Context, conn driver.Conn, table string) (bool, error) {
	var n uint8
	if err := conn.QueryRow(ctx, "EXISTS TABLE "+table).Scan(&n); err != nil {
		return false, err
	}
	return n == 1, nil
}
