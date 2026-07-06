package parquet

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cvhariharan/checkpost/internal/resultquery"
	"github.com/cvhariharan/checkpost/internal/results"
	"github.com/google/uuid"
	_ "github.com/marcboeker/go-duckdb/v2"
)

// dialect renders filter terms against real Parquet columns read through DuckDB.
var dialect = resultquery.SQLDialect{Column: lowerColumnExpr, LikeSuffix: ` ESCAPE '\'`}

var exportSlots = make(chan struct{}, 2)

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

func (r *Reader) Export(ctx context.Context, w io.Writer, req results.ExportRequest) error {
	if req.Format != "csv" {
		return fmt.Errorf("unsupported export format %q", req.Format)
	}
	select {
	case exportSlots <- struct{}{}:
		defer func() { <-exportSlots }()
	case <-ctx.Done():
		return ctx.Err()
	}

	readExpr, sources, err := r.exportReadExpr(req.Sources)
	if err != nil {
		return err
	}
	if readExpr == "" || (len(req.Columns) == 0 && !req.IncludeHost && !req.IncludeMachine) {
		return writeCSVHeader(w, req.IncludeHost, req.IncludeMachine, req.Columns)
	}

	selectCols := buildSelectList(req.Columns)
	if req.IncludeHost {
		selectCols = "h.hostname"
		if len(req.Columns) > 0 {
			selectCols += ", " + buildSelectList(req.Columns)
		}
	} else if req.IncludeMachine {
		selectCols = "h.hostname, p.unix_time AS last_seen"
		if len(req.Columns) > 0 {
			selectCols += ", " + buildSelectList(req.Columns)
		}
	}
	filterSQL, filterArgs, err := resultquery.RenderFilters(dialect, req.Columns, req.Filters, nil, "p.node_id")
	if err != nil {
		return err
	}
	if len(filterArgs) > 0 {
		return errors.New("filtered export is not supported")
	}
	hostnames := req.Hostnames
	if len(hostnames) == 0 {
		hostnames = sources
	}
	query := exportSelectSQL(readExpr, selectCols, req.Snapshot, req.IncludeHost || req.IncludeMachine, hostnames, filterSQL)

	// DuckDB COPY writes to a file path. When the destination is already a file
	// (the HTTP-export path), COPY straight into it so the result set hits disk
	// once; otherwise stage through a temp file and stream the bytes back.
	if f, ok := w.(*os.File); ok {
		return r.copyToCSV(ctx, query, f.Name())
	}

	tmp, err := os.CreateTemp("", "ck-export-*.csv")
	if err != nil {
		return fmt.Errorf("create export temp file: %w", err)
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close export temp file: %w", err)
	}
	defer os.Remove(tmpPath)

	if err := r.copyToCSV(ctx, query, tmpPath); err != nil {
		return err
	}
	f, err := os.Open(tmpPath)
	if err != nil {
		return fmt.Errorf("open export temp file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(w, f); err != nil {
		return fmt.Errorf("copy export: %w", err)
	}
	return nil
}

func (r *Reader) copyToCSV(ctx context.Context, query, path string) error {
	stmt := fmt.Sprintf("COPY (%s) TO '%s' (FORMAT CSV, HEADER)", query, escapeSQLLiteral(path))
	if _, err := r.db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("duckdb export: %w", err)
	}
	return nil
}

func (r *Reader) Read(ctx context.Context, scheduleUUID uuid.UUID, sqlVersion int32, columns []string, opts results.ReadOptions) (results.Result, error) {
	if opts.Limit <= 0 {
		opts.Limit = 1000
	}

	glob := r.partitionGlob(scheduleUUID, sqlVersion, opts.NodeID)

	matches, err := filepath.Glob(glob)
	if err != nil {
		return results.Result{}, fmt.Errorf("glob partition: %w", err)
	}
	if len(matches) == 0 {
		return results.Result{Columns: columns}, nil
	}

	selectCols := buildSelectList(columns)
	if selectCols != "" {
		selectCols = ", " + selectCols
	}
	nodeIDColumn := "node_id"
	if opts.Snapshot {
		nodeIDColumn = "p.node_id"
	}
	filterSQL, filterArgs, err := resultquery.RenderFilters(dialect, columns, opts.Filters, opts.NodeIDs, nodeIDColumn)
	if err != nil {
		return results.Result{}, err
	}
	var query string
	if opts.Snapshot {
		query = fmt.Sprintf(`
WITH %s
SELECT p.node_id, p.unix_time, p.action%s
FROM %s AS p
%s
WHERE %s%s
LIMIT %d OFFSET %d`, snapshotLatestCTE("?"), selectCols, readParquet("?"), snapshotJoinClause, snapshotWhere, filterSQL, opts.Limit, opts.Offset)
	} else {
		query = fmt.Sprintf(`
WITH %s
SELECT node_id, unix_time, action%s
FROM ranked
WHERE %s%s
LIMIT %d OFFSET %d`, rankedCTE("?", "*"), selectCols, dedupWhere, filterSQL, opts.Limit, opts.Offset)
	}

	args := []any{glob}
	if opts.Snapshot {
		args = append(args, glob)
	}
	args = append(args, filterArgs...)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return results.Result{}, fmt.Errorf("duckdb read: %w", err)
	}
	defer rows.Close()

	out := make([]results.ResultRow, 0, opts.Limit)
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
			return results.Result{}, fmt.Errorf("scan row: %w", err)
		}
		valMap := make(map[string]string, len(columns))
		for i, col := range columns {
			s := scanInto[3+i].(*sql.NullString)
			if s.Valid {
				valMap[col] = s.String
			}
		}
		out = append(out, results.ResultRow{
			NodeID:   nodeID.Int64,
			UnixTime: unixTime.Time,
			Action:   action.String,
			Values:   valMap,
		})
	}
	if err := rows.Err(); err != nil {
		return results.Result{}, fmt.Errorf("iterate rows: %w", err)
	}

	total, err := r.count(ctx, glob, opts.Snapshot, columns, filterSQL, filterArgs)
	if err != nil {
		return results.Result{}, err
	}

	return results.Result{Columns: columns, Rows: out, Total: total}, nil
}

func (r *Reader) count(ctx context.Context, glob string, snapshot bool, columns []string, filterSQL string, filterArgs []any) (int, error) {
	var query string
	if snapshot {
		query = fmt.Sprintf(`
WITH %s
SELECT count(*)
FROM %s AS p
%s
WHERE %s%s`, snapshotLatestCTE("?"), readParquet("?"), snapshotJoinClause, snapshotWhere, filterSQL)
	} else {
		selectCols := buildSelectList(columns)
		if selectCols != "" {
			selectCols = ", " + selectCols
		}
		query = fmt.Sprintf(`
WITH %s
SELECT count(*) FROM ranked WHERE %s%s`,
			rankedCTE("?", "node_id, row_hash, unix_time, action"+selectCols), dedupWhere, filterSQL)
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

func lowerColumnExpr(col string) string {
	return `lower(COALESCE(CAST("` + sanitizeIdent(col) + `" AS VARCHAR), ''))`
}

func (r *Reader) partitionGlob(scheduleUUID uuid.UUID, sqlVersion int32, nodeID int64) string {
	if nodeID == 0 {
		return filepath.Join(versionDir(r.root, scheduleUUID, sqlVersion), "host=*", "*.parquet")
	}
	return filepath.Join(partitionDir(r.root, PartitionKey{SourceUUID: scheduleUUID, SQLVersion: sqlVersion, NodeID: nodeID}), "*.parquet")
}

// The following helpers centralise the definition of "the current row" so the
// snapshot-selection and dedup rules live in one place instead of being copied
// across Read, count, and Export. src is the read_parquet source argument —
// either a "?" placeholder (parameterised reads) or an inlined literal glob
// list (exports).

func readParquet(src string) string {
	return fmt.Sprintf("read_parquet(%s, union_by_name=true, hive_partitioning=true)", src)
}

// snapshotLatestCTE selects, per node, the newest snapshot ingestion.
func snapshotLatestCTE(src string) string {
	return fmt.Sprintf(`latest AS (
    SELECT node_id, max(ingested_at) AS t
    FROM %s
    WHERE action = 'snapshot'
    GROUP BY node_id
)`, readParquet(src))
}

// rankedCTE ranks rows per (node_id, row_hash) newest-first so rn = 1 is the
// live row. projection is the column list carried through the window ("*" for
// full-row reads, an explicit list for count).
func rankedCTE(src, projection string) string {
	return fmt.Sprintf(`ranked AS (
    SELECT %s, row_number() OVER (
        PARTITION BY node_id, row_hash
        ORDER BY unix_time DESC, ingested_at DESC
    ) AS rn
    FROM %s
)`, projection, readParquet(src))
}

const (
	snapshotJoinClause = "JOIN latest ON latest.node_id = p.node_id AND latest.t = p.ingested_at"
	snapshotWhere      = "p.action = 'snapshot'"
	dedupWhere         = "rn = 1 AND action <> 'removed'"
)

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

func (r *Reader) exportReadExpr(sources []results.ExportSource) (string, []results.ExportSource, error) {
	globs := make([]string, 0, len(sources))
	matched := make([]results.ExportSource, 0, len(sources))
	for _, source := range sources {
		glob := r.partitionGlob(source.SourceUUID, source.SQLVersion, source.NodeID)
		matches, err := filepath.Glob(glob)
		if err != nil {
			return "", nil, fmt.Errorf("glob export partition: %w", err)
		}
		if len(matches) == 0 {
			continue
		}
		globs = append(globs, glob)
		matched = append(matched, source)
	}
	if len(globs) == 0 {
		return "", nil, nil
	}
	if len(globs) == 1 {
		return "'" + escapeSQLLiteral(globs[0]) + "'", matched, nil
	}
	quoted := make([]string, 0, len(globs))
	for _, glob := range globs {
		quoted = append(quoted, "'"+escapeSQLLiteral(glob)+"'")
	}
	return "[" + strings.Join(quoted, ", ") + "]", matched, nil
}

func exportSelectSQL(readExpr, selectCols string, snapshot, includeHost bool, sources []results.ExportSource, filterSQL string) string {
	hostCTE := ""
	hostJoin := ""
	if includeHost {
		values := make([]string, 0, len(sources))
		for _, source := range sources {
			values = append(values, fmt.Sprintf("(%d, '%s')", source.NodeID, escapeSQLLiteral(source.Hostname)))
		}
		hostCTE = ", hosts(node_id, hostname) AS (VALUES " + strings.Join(values, ", ") + ")"
		hostJoin = "\nLEFT JOIN hosts h ON h.node_id = p.node_id"
	}
	if snapshot {
		return fmt.Sprintf(`
WITH %s%s
SELECT %s
FROM %s AS p
%s%s
WHERE %s%s`, snapshotLatestCTE(readExpr), hostCTE, selectCols, readParquet(readExpr), snapshotJoinClause, hostJoin, snapshotWhere, filterSQL)
	}
	return fmt.Sprintf(`
WITH %s%s
SELECT %s
FROM ranked AS p%s
WHERE %s%s`, rankedCTE(readExpr, "*"), hostCTE, selectCols, hostJoin, dedupWhere, filterSQL)
}

func writeCSVHeader(w io.Writer, includeHost, includeMachine bool, columns []string) error {
	header := append([]string(nil), columns...)
	if includeHost {
		header = append([]string{"hostname"}, header...)
	} else if includeMachine {
		header = append([]string{"hostname", "last_seen"}, header...)
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(header); err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
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
