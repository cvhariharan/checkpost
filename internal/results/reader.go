package results

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/cvhariharan/checkpost/internal/resultquery"
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
	// Multiple connections so user-facing reads don't queue behind a
	// multi-second compaction COPY running on the worker pool.
	db.SetMaxOpenConns(8)
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
	NodeIDs  []int64
	Snapshot bool
	Limit    int
	Offset   int
	Filters  []resultquery.Term
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
	nodeIDColumn := "node_id"
	if opts.Snapshot {
		nodeIDColumn = "p.node_id"
	}
	filterSQL, filterArgs, err := buildFilterSQL(columns, opts.Filters, opts.NodeIDs, nodeIDColumn)
	if err != nil {
		return Result{}, err
	}
	var query string
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
WHERE p.action = 'snapshot'%s
LIMIT %d OFFSET %d`, selectCols, filterSQL, opts.Limit, opts.Offset)
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
WHERE rn = 1 AND action <> 'removed'%s
LIMIT %d OFFSET %d`, selectCols, filterSQL, opts.Limit, opts.Offset)
	}

	args := []any{glob}
	if opts.Snapshot {
		args = append(args, glob)
	}
	args = append(args, filterArgs...)

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

	total, err := r.count(ctx, glob, opts.Snapshot, columns, filterSQL, filterArgs)
	if err != nil {
		return Result{}, err
	}

	return Result{Columns: columns, Rows: out, Total: total}, nil
}

func (r *Reader) count(ctx context.Context, glob string, snapshot bool, columns []string, filterSQL string, filterArgs []any) (int, error) {
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
WHERE p.action = 'snapshot'` + filterSQL
	} else {
		selectCols := buildSelectList(columns)
		if selectCols != "" {
			selectCols = ", " + selectCols
		}
		query = `
WITH ranked AS (
    SELECT node_id, row_hash, unix_time, action` + selectCols + `, row_number() OVER (
        PARTITION BY node_id, row_hash
        ORDER BY unix_time DESC, ingested_at DESC
    ) AS rn
    FROM read_parquet(?, union_by_name=true, hive_partitioning=true)
)
SELECT count(*) FROM ranked WHERE rn = 1 AND action <> 'removed'` + filterSQL
	}
	var n int
	args := []any{glob}
	if snapshot {
		args = append(args, glob)
	}
	args = append(args, filterArgs...)
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("count: %w", err)
	}
	return n, nil
}

func buildFilterSQL(columns []string, filters []resultquery.Term, nodeIDs []int64, nodeIDColumn string) (string, []any, error) {
	if nodeIDColumn == "" {
		nodeIDColumn = "node_id"
	}
	var parts []string
	var args []any
	if len(nodeIDs) > 0 {
		placeholders := make([]string, 0, len(nodeIDs))
		for _, id := range nodeIDs {
			placeholders = append(placeholders, "?")
			args = append(args, id)
		}
		parts = append(parts, nodeIDColumn+" IN ("+strings.Join(placeholders, ", ")+")")
	}
	for _, filter := range filters {
		sql, sqlArgs, err := termSQL(columns, filter)
		if err != nil {
			return "", nil, err
		}
		if sql == "" {
			continue
		}
		parts = append(parts, sql)
		args = append(args, sqlArgs...)
	}
	if len(parts) == 0 {
		return "", nil, nil
	}
	return " AND " + strings.Join(parts, " AND "), args, nil
}

func termSQL(columns []string, term resultquery.Term) (string, []any, error) {
	if term.Field == "" {
		if len(columns) == 0 {
			return "", nil, fmt.Errorf("no searchable columns recorded for this schedule yet")
		}
		preds := make([]string, 0, len(columns))
		args := make([]any, 0, len(columns))
		for _, col := range columns {
			preds = append(preds, lowerColumnExpr(col)+" LIKE ? ESCAPE '\\'")
			args = append(args, "%"+escapeLike(strings.ToLower(term.Value))+"%")
		}
		return "(" + strings.Join(preds, " OR ") + ")", args, nil
	}
	if term.Field == resultquery.FieldMachine {
		return "", nil, nil
	}
	if term.Field == resultquery.FieldLastSeen {
		if term.Op != resultquery.OpGTE && term.Op != resultquery.OpLTE {
			return "", nil, fmt.Errorf("last_seen only supports >= and <=")
		}
		t, err := time.Parse(time.RFC3339, term.Value)
		if err != nil {
			return "", nil, fmt.Errorf("parse last_seen timestamp: %w", err)
		}
		if term.Op == resultquery.OpGTE {
			return "unix_time >= ?", []any{t}, nil
		}
		return "unix_time <= ?", []any{t}, nil
	}
	if !hasColumn(columns, term.Field) {
		return "", nil, fmt.Errorf("unknown field %q", term.Field)
	}
	expr := lowerColumnExpr(term.Field)
	switch term.Op {
	case resultquery.OpContains:
		return expr + " LIKE ? ESCAPE '\\'", []any{"%" + escapeLike(strings.ToLower(term.Value)) + "%"}, nil
	case resultquery.OpEqual:
		return expr + " = ?", []any{strings.ToLower(term.Value)}, nil
	default:
		return "", nil, fmt.Errorf("%s does not support %s", term.Field, term.Op)
	}
}

func hasColumn(columns []string, name string) bool {
	for _, col := range columns {
		if col == name {
			return true
		}
	}
	return false
}

func lowerColumnExpr(col string) string {
	return `lower(COALESCE(CAST("` + sanitizeIdent(col) + `" AS VARCHAR), ''))`
}

func escapeLike(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r == '%' || r == '_' || r == '\\' {
			b.WriteRune('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
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
