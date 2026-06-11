package resultquery

import (
	"testing"
)

// testDialect renders columns verbatim with a visible ESCAPE-like suffix so
// tests can assert both the column accessor and the LIKE suffix are applied.
var testDialect = SQLDialect{
	Column:     func(col string) string { return "C(" + col + ")" },
	LikeSuffix: " ESC",
}

func TestRenderFilters(t *testing.T) {
	cols := []string{"name", "path"}

	tests := []struct {
		name     string
		columns  []string
		filters  []Term
		nodeIDs  []int64
		nodeCol  string
		wantSQL  string
		wantArgs []any
		wantErr  bool
	}{
		{
			name:    "no filters",
			columns: cols,
		},
		{
			name:     "node ids only",
			columns:  cols,
			nodeIDs:  []int64{4, 9},
			nodeCol:  "p.node_id",
			wantSQL:  " AND p.node_id IN (?, ?)",
			wantArgs: []any{int64(4), int64(9)},
		},
		{
			name:     "column contains is lowercased, escaped, suffixed",
			columns:  cols,
			filters:  []Term{{Field: "path", Op: OpContains, Value: "Bin%"}},
			wantSQL:  ` AND C(path) LIKE ? ESC`,
			wantArgs: []any{`%bin\%%`},
		},
		{
			name:     "column equal has no suffix",
			columns:  cols,
			filters:  []Term{{Field: "path", Op: OpEqual, Value: "BIN"}},
			wantSQL:  " AND C(path) = ?",
			wantArgs: []any{"bin"},
		},
		{
			name:     "free text fans out across columns with suffix",
			columns:  cols,
			filters:  []Term{{Field: "", Op: OpContains, Value: "x"}},
			wantSQL:  " AND (C(name) LIKE ? ESC OR C(path) LIKE ? ESC)",
			wantArgs: []any{"%x%", "%x%"},
		},
		{
			name:    "last_seen gte (no suffix)",
			columns: cols,
			filters: []Term{{Field: FieldLastSeen, Op: OpGTE, Value: "2026-01-02T15:04:05Z"}},
			wantSQL: " AND unix_time >= ?",
		},
		{
			name:    "machine term is skipped here",
			columns: cols,
			filters: []Term{{Field: FieldMachine, Op: OpEqual, Value: "host1"}},
		},
		{
			name:    "unknown field errors",
			columns: cols,
			filters: []Term{{Field: "nope", Op: OpEqual, Value: "x"}},
			wantErr: true,
		},
		{
			name:    "free text with no columns errors",
			filters: []Term{{Field: "", Op: OpContains, Value: "x"}},
			wantErr: true,
		},
		{
			name:    "last_seen with bad op errors",
			columns: cols,
			filters: []Term{{Field: FieldLastSeen, Op: OpContains, Value: "x"}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := RenderFilters(testDialect, tt.columns, tt.filters, tt.nodeIDs, tt.nodeCol)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got sql=%q", sql)
				}
				return
			}
			if err != nil {
				t.Fatalf("RenderFilters: %v", err)
			}
			if sql != tt.wantSQL {
				t.Errorf("sql = %q, want %q", sql, tt.wantSQL)
			}
			// last_seen produces a time.Time arg we don't pin; only check the rest.
			if tt.wantArgs != nil {
				if len(args) != len(tt.wantArgs) {
					t.Fatalf("args = %v, want %v", args, tt.wantArgs)
				}
				for i, want := range tt.wantArgs {
					if args[i] != want {
						t.Errorf("arg[%d] = %#v, want %#v", i, args[i], want)
					}
				}
			}
		})
	}
}

func TestEscapeLike(t *testing.T) {
	if got := EscapeLike(`50%_v\x`); got != `50\%\_v\\x` {
		t.Errorf("EscapeLike = %q", got)
	}
}

func TestHasColumn(t *testing.T) {
	cols := []string{"a", "b"}
	if !HasColumn(cols, "b") {
		t.Error("HasColumn(b) = false, want true")
	}
	if HasColumn(cols, "c") {
		t.Error("HasColumn(c) = true, want false")
	}
}
