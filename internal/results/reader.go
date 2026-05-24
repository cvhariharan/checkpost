package results

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/marcboeker/go-duckdb/v2"
)

type Reader struct {
	root string
	db   *sql.DB
}

func NewReader(root, duckdbPath string) (*Reader, error) {
	dsn := duckdbPath
	db, err := sql.Open("duckdb", dsn)
	if err != nil {
		return nil, fmt.Errorf("open duckdb: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping duckdb: %w", err)
	}
	return &Reader{root: root, db: db}, nil
}

func (r *Reader) Close() error {
	return r.db.Close()
}

type ReadOptions struct {
	NodeID   int64
	Snapshot bool
	Limit    int
	Offset   int
}

type Result struct {
	Columns []string
	Rows    []ResultRow
	Total   int
}

type ResultRow struct {
	NodeID   int64
	UnixTime time.Time
	Action   string
	Values   map[string]string
}

func (r *Reader) Read(ctx context.Context, scheduleUUID uuid.UUID, sqlVersion int32, columns []string, opts ReadOptions) (Result, error) {
	if opts.Limit <= 0 {
		opts.Limit = 1000
	}

	glob := r.partitionGlob(scheduleUUID, sqlVersion, opts.NodeID)

	matches, err := filepath.Glob(glob)
	if err != nil {
		return Result{}, fmt.Errorf("glob partition: %w", err)
	}
	if len(matches) == 0 {
		return Result{Columns: columns}, nil
	}

	selectCols := buildSelectList(columns)
	if selectCols != "" {
		selectCols = ", " + selectCols
	}
	var (
		query string
		total int
	)
	if opts.Snapshot {
		query = fmt.Sprintf(`
WITH latest AS (
    SELECT node_id, max(ingested_at) AS t
    FROM read_parquet(?, union_by_name=true, hive_partitioning=true)
    WHERE action = 'snapshot'
    GROUP BY node_id
)
SELECT p.node_id, p.unix_time, p.action%s
FROM read_parquet(?, union_by_name=true, hive_partitioning=true) AS p
JOIN latest ON latest.node_id = p.node_id AND latest.t = p.ingested_at
WHERE p.action = 'snapshot'
LIMIT %d OFFSET %d`, selectCols, opts.Limit, opts.Offset)
	} else {
		query = fmt.Sprintf(`
WITH ranked AS (
    SELECT *, row_number() OVER (
        PARTITION BY node_id, row_hash
        ORDER BY unix_time DESC, ingested_at DESC
    ) AS rn
    FROM read_parquet(?, union_by_name=true, hive_partitioning=true)
)
SELECT node_id, unix_time, action%s
FROM ranked
WHERE rn = 1 AND action <> 'removed'
LIMIT %d OFFSET %d`, selectCols, opts.Limit, opts.Offset)
	}

	args := []any{glob}
	if opts.Snapshot {
		args = append(args, glob)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return Result{}, fmt.Errorf("duckdb read: %w", err)
	}
	defer rows.Close()

	out := make([]ResultRow, 0, opts.Limit)
	values := make([]any, 3+len(columns))
	scanInto := make([]any, len(values))
	for i := range values {
		scanInto[i] = new(sql.NullString)
	}
	var nodeID sql.NullInt64
	var unixTime sql.NullTime
	var action sql.NullString
	scanInto[0] = &nodeID
	scanInto[1] = &unixTime
	scanInto[2] = &action

	for rows.Next() {
		for i := 3; i < len(scanInto); i++ {
			s := scanInto[i].(*sql.NullString)
			*s = sql.NullString{}
		}
		if err := rows.Scan(scanInto...); err != nil {
			return Result{}, fmt.Errorf("scan row: %w", err)
		}
		valMap := make(map[string]string, len(columns))
		for i, col := range columns {
			s := scanInto[3+i].(*sql.NullString)
			if s.Valid {
				valMap[col] = s.String
			}
		}
		out = append(out, ResultRow{
			NodeID:   nodeID.Int64,
			UnixTime: unixTime.Time,
			Action:   action.String,
			Values:   valMap,
		})
	}
	if err := rows.Err(); err != nil {
		return Result{}, fmt.Errorf("iterate rows: %w", err)
	}

	total, err = r.count(ctx, glob, opts.Snapshot)
	if err != nil {
		return Result{}, err
	}

	return Result{Columns: columns, Rows: out, Total: total}, nil
}

func (r *Reader) count(ctx context.Context, glob string, snapshot bool) (int, error) {
	var query string
	if snapshot {
		query = `
WITH latest AS (
    SELECT node_id, max(ingested_at) AS t
    FROM read_parquet(?, union_by_name=true, hive_partitioning=true)
    WHERE action = 'snapshot'
    GROUP BY node_id
)
SELECT count(*)
FROM read_parquet(?, union_by_name=true, hive_partitioning=true) AS p
JOIN latest ON latest.node_id = p.node_id AND latest.t = p.ingested_at
WHERE p.action = 'snapshot'`
	} else {
		query = `
WITH ranked AS (
    SELECT node_id, row_hash, action, row_number() OVER (
        PARTITION BY node_id, row_hash
        ORDER BY unix_time DESC, ingested_at DESC
    ) AS rn
    FROM read_parquet(?, union_by_name=true, hive_partitioning=true)
)
SELECT count(*) FROM ranked WHERE rn = 1 AND action <> 'removed'`
	}
	var n int
	args := []any{glob}
	if snapshot {
		args = append(args, glob)
	}
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("count: %w", err)
	}
	return n, nil
}

func (r *Reader) partitionGlob(scheduleUUID uuid.UUID, sqlVersion int32, nodeID int64) string {
	if nodeID == 0 {
		return filepath.Join(versionDir(r.root, scheduleUUID, sqlVersion), "host=*", "*.parquet")
	}
	return filepath.Join(partitionDir(r.root, PartitionKey{ScheduleUUID: scheduleUUID, SQLVersion: sqlVersion, NodeID: nodeID}), "*.parquet")
}

func buildSelectList(columns []string) string {
	if len(columns) == 0 {
		return ""
	}
	parts := make([]string, 0, len(columns))
	for _, c := range columns {
		parts = append(parts, fmt.Sprintf("%q", sanitizeIdent(c)))
	}
	return strings.Join(parts, ", ")
}

func sanitizeIdent(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "_invalid"
	}
	return b.String()
}
