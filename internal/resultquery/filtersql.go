package resultquery

import (
	"fmt"
	"strings"
	"time"
)

// SQLDialect captures the backend-specific bits of rendering a filter term:
// how to address a named column case-insensitively, and the suffix appended to
// a LIKE predicate (e.g. an ESCAPE clause). Placeholders are always "?".
type SQLDialect struct {
	Column     func(col string) string
	LikeSuffix string
}

// RenderFilters renders the node-id IN list and query terms as a " AND ..."
// suffix, or "" when there is nothing to filter. nodeIDColumn names the column
// the IN list targets (e.g. "node_id" or "p.node_id").
func RenderFilters(d SQLDialect, columns []string, filters []Term, nodeIDs []int64, nodeIDColumn string) (string, []any, error) {
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
		sql, sqlArgs, err := termSQL(d, columns, filter)
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

func termSQL(d SQLDialect, columns []string, term Term) (string, []any, error) {
	if term.Field == "" {
		if len(columns) == 0 {
			return "", nil, fmt.Errorf("no searchable columns recorded for this schedule yet")
		}
		preds := make([]string, 0, len(columns))
		args := make([]any, 0, len(columns))
		for _, col := range columns {
			preds = append(preds, d.Column(col)+" LIKE ?"+d.LikeSuffix)
			args = append(args, "%"+EscapeLike(strings.ToLower(term.Value))+"%")
		}
		return "(" + strings.Join(preds, " OR ") + ")", args, nil
	}
	if term.Field == FieldMachine {
		// machine: terms are resolved to node IDs upstream (nodeIDs).
		return "", nil, nil
	}
	if term.Field == FieldLastSeen {
		if term.Op != OpGTE && term.Op != OpLTE {
			return "", nil, fmt.Errorf("last_seen only supports >= and <=")
		}
		t, err := time.Parse(time.RFC3339, term.Value)
		if err != nil {
			return "", nil, fmt.Errorf("parse last_seen timestamp: %w", err)
		}
		if term.Op == OpGTE {
			return "unix_time >= ?", []any{t}, nil
		}
		return "unix_time <= ?", []any{t}, nil
	}
	if !HasColumn(columns, term.Field) {
		return "", nil, fmt.Errorf("unknown field %q", term.Field)
	}
	expr := d.Column(term.Field)
	switch term.Op {
	case OpContains:
		return expr + " LIKE ?" + d.LikeSuffix, []any{"%" + EscapeLike(strings.ToLower(term.Value)) + "%"}, nil
	case OpEqual:
		return expr + " = ?", []any{strings.ToLower(term.Value)}, nil
	default:
		return "", nil, fmt.Errorf("%s does not support %s", term.Field, term.Op)
	}
}

func HasColumn(columns []string, name string) bool {
	for _, col := range columns {
		if col == name {
			return true
		}
	}
	return false
}

// EscapeLike escapes LIKE metacharacters with backslash, the default escape
// character for both DuckDB and ClickHouse.
func EscapeLike(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r == '%' || r == '_' || r == '\\' {
			b.WriteRune('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
