// Package clickhouse implements a result Sink that inserts rows into a
// ClickHouse table. osquery's dynamic columns are stored in a Map(String,
// String) so the schema is stable across schedules.
package clickhouse

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/cvhariharan/checkpost/internal/results"
	"github.com/google/uuid"
)

const (
	queueSize     = 8192
	maxBatch      = 1000
	flushInterval = 5 * time.Second
)

type row struct {
	scheduleUUID uuid.UUID
	scheduleName string
	sqlVersion   int32
	nodeID       int64
	unixTime     time.Time
	calendarTime string
	action       string
	rowHash      string
	columns      map[string]string
}

// Sink buffers rows and inserts them in batches from a single goroutine.
type Sink struct {
	conn    driver.Conn
	table   string
	in      chan row
	done    chan struct{}
	logger  *slog.Logger
	dropped atomic.Uint64
}

// New connects to ClickHouse, ensures the results table exists, and starts the
// batch inserter. table defaults to "query_results"; ttlDays > 0 sets a TTL.
func New(dsn, table string, ttlDays int, logger *slog.Logger) (*Sink, error) {
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
		done:   make(chan struct{}),
		logger: logger.WithGroup("results.clickhouse"),
	}
	go s.run()
	return s, nil
}

func (s *Sink) Name() string { return "clickhouse" }

func (s *Sink) Submit(ctx context.Context, batch results.Batch) error {
	for _, r := range batch.Rows {
		select {
		case s.in <- row{
			scheduleUUID: batch.ScheduleUUID,
			scheduleName: batch.ScheduleName,
			sqlVersion:   batch.SQLVersion,
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

func (s *Sink) DeleteSchedule(ctx context.Context, scheduleUUID uuid.UUID) error {
	return s.conn.Exec(ctx,
		fmt.Sprintf("ALTER TABLE %s DELETE WHERE schedule_uuid = {id:UUID}", s.table),
		clickhouse.Named("id", scheduleUUID),
	)
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

func ensureTable(ctx context.Context, conn driver.Conn, table string, ttlDays int) error {
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
	return conn.Exec(ctx, ddl)
}
