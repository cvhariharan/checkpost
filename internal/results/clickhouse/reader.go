package clickhouse

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cvhariharan/checkpost/internal/resultquery"
	"github.com/cvhariharan/checkpost/internal/results"
	"github.com/google/uuid"
)

// dialect renders filter terms against the ClickHouse Map(String, String)
// `columns`. ClickHouse uses backslash as the default LIKE escape, so no ESCAPE
// suffix is needed.
var dialect = resultquery.SQLDialect{Column: mapColumnExpr}

// Read satisfies results.Reader for the in-app results browser, mirroring the
// Parquet reader's snapshot/differential semantics so the two are
// interchangeable. Dynamic osquery columns live in the Map(String, String)
// `columns`, so requested columns are projected out of that map in Go.
func (s *Sink) Read(ctx context.Context, scheduleUUID uuid.UUID, sqlVersion int32, columns []string, opts results.ReadOptions) (results.Result, error) {
	if opts.Limit <= 0 {
		opts.Limit = 1000
	}

	query, args, err := s.buildReadQuery(scheduleUUID, sqlVersion, columns, opts, false)
	if err != nil {
		return results.Result{}, err
	}

	rows, err := s.conn.Query(ctx, query, args...)
	if err != nil {
		return results.Result{}, fmt.Errorf("clickhouse read: %w", err)
	}
	defer rows.Close()

	out := make([]results.ResultRow, 0, opts.Limit)
	for rows.Next() {
		var (
			nodeID   int64
			unixTime time.Time
			action   string
			cols     map[string]string
		)
		if err := rows.Scan(&nodeID, &unixTime, &action, &cols); err != nil {
			return results.Result{}, fmt.Errorf("scan row: %w", err)
		}
		valMap := make(map[string]string, len(columns))
		for _, col := range columns {
			if v, ok := cols[col]; ok {
				valMap[col] = v
			}
		}
		out = append(out, results.ResultRow{
			NodeID:   nodeID,
			UnixTime: unixTime,
			Action:   action,
			Values:   valMap,
		})
	}
	if err := rows.Err(); err != nil {
		return results.Result{}, fmt.Errorf("iterate rows: %w", err)
	}

	total, err := s.count(ctx, scheduleUUID, sqlVersion, columns, opts)
	if err != nil {
		return results.Result{}, err
	}

	return results.Result{Columns: columns, Rows: out, Total: total}, nil
}

func (s *Sink) count(ctx context.Context, scheduleUUID uuid.UUID, sqlVersion int32, columns []string, opts results.ReadOptions) (int, error) {
	query, args, err := s.buildReadQuery(scheduleUUID, sqlVersion, columns, opts, true)
	if err != nil {
		return 0, err
	}
	var n uint64
	if err := s.conn.QueryRow(ctx, query, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("count: %w", err)
	}
	return int(n), nil
}

// buildReadQuery renders the snapshot or differential query (countOnly projects
// count(*) and drops pagination).
func (s *Sink) buildReadQuery(scheduleUUID uuid.UUID, sqlVersion int32, columns []string, opts results.ReadOptions, countOnly bool) (string, []any, error) {
	if opts.Snapshot {
		latestScope, latestArgs := scopeClause("", scheduleUUID, sqlVersion, opts.NodeID)
		outerScope, outerArgs := scopeClause("p.", scheduleUUID, sqlVersion, opts.NodeID)
		filterSQL, filterArgs, err := resultquery.RenderFilters(dialect, columns, opts.Filters, opts.NodeIDs, "p.node_id")
		if err != nil {
			return "", nil, err
		}
		projection, tail := selectAndTail(countOnly, "p.node_id, p.unix_time, p.action, p.columns", "p.node_id, p.unix_time", opts)
		query := fmt.Sprintf(`WITH latest AS (
    SELECT node_id, max(ingested_at) AS t
    FROM %s
    WHERE %s AND action = 'snapshot'
    GROUP BY node_id
)
SELECT %s
FROM %s AS p
INNER JOIN latest ON latest.node_id = p.node_id AND latest.t = p.ingested_at
WHERE %s AND p.action = 'snapshot'%s%s`,
			s.table, latestScope, projection, s.table, outerScope, filterSQL, tail)
		args := make([]any, 0, len(latestArgs)+len(outerArgs)+len(filterArgs))
		args = append(args, latestArgs...)
		args = append(args, outerArgs...)
		args = append(args, filterArgs...)
		return query, args, nil
	}

	innerScope, innerArgs := scopeClause("", scheduleUUID, sqlVersion, opts.NodeID)
	filterSQL, filterArgs, err := resultquery.RenderFilters(dialect, columns, opts.Filters, opts.NodeIDs, "node_id")
	if err != nil {
		return "", nil, err
	}
	projection, tail := selectAndTail(countOnly, "node_id, unix_time, action, columns", "node_id, unix_time", opts)
	query := fmt.Sprintf(`WITH ranked AS (
    SELECT node_id, unix_time, action, columns, row_number() OVER (
        PARTITION BY node_id, row_hash
        ORDER BY unix_time DESC, ingested_at DESC
    ) AS rn
    FROM %s
    WHERE %s
)
SELECT %s
FROM ranked
WHERE rn = 1 AND action != 'removed'%s%s`,
		s.table, innerScope, projection, filterSQL, tail)
	args := make([]any, 0, len(innerArgs)+len(filterArgs))
	args = append(args, innerArgs...)
	args = append(args, filterArgs...)
	return query, args, nil
}

func selectAndTail(countOnly bool, projection, orderBy string, opts results.ReadOptions) (string, string) {
	if countOnly {
		return "count(*)", ""
	}
	return projection, fmt.Sprintf("\nORDER BY %s\nLIMIT %d OFFSET %d", orderBy, opts.Limit, opts.Offset)
}

// scopeClause builds the schedule scope, optionally narrowed to one node, with
// column references prefixed (e.g. "p." inside the snapshot join).
func scopeClause(prefix string, scheduleUUID uuid.UUID, sqlVersion int32, nodeID int64) (string, []any) {
	clause := fmt.Sprintf("%sschedule_uuid = ? AND %ssql_version = ?", prefix, prefix)
	args := []any{scheduleUUID, sqlVersion}
	if nodeID != 0 {
		clause += fmt.Sprintf(" AND %snode_id = ?", prefix)
		args = append(args, nodeID)
	}
	return clause, args
}

// mapColumnExpr is a case-insensitive accessor for a dynamic column in the
// ClickHouse Map; missing keys read back as empty. The key is inlined as an
// escaped literal (map subscripts take a value), sourced from the validated schema.
func mapColumnExpr(col string) string {
	return "lower(columns[" + quoteMapKey(col) + "])"
}

func quoteMapKey(col string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range col {
		if r == '\'' || r == '\\' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('\'')
	return b.String()
}
