package resultquery

import "testing"

func TestParseTerms(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []Term
	}{
		{
			name:  "global terms",
			input: "chrome helper",
			want: []Term{
				{Op: OpContains, Value: "chrome"},
				{Op: OpContains, Value: "helper"},
			},
		},
		{
			name:  "field filters",
			input: `name:osqueryd uid=0 path:"Program Files"`,
			want: []Term{
				{Field: "name", Op: OpContains, Value: "osqueryd"},
				{Field: "uid", Op: OpEqual, Value: "0"},
				{Field: "path", Op: OpContains, Value: "Program Files"},
			},
		},
		{
			name:  "timestamp comparisons",
			input: "last_seen>=2026-05-24T00:00:00Z last_seen<=2026-05-24T12:00:00Z",
			want: []Term{
				{Field: "last_seen", Op: OpGTE, Value: "2026-05-24T00:00:00Z"},
				{Field: "last_seen", Op: OpLTE, Value: "2026-05-24T12:00:00Z"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d terms, got %d: %+v", len(tt.want), len(got), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("term %d: expected %+v, got %+v", i, tt.want[i], got[i])
				}
			}
		})
	}
}

func TestParseRejectsInvalidSyntax(t *testing.T) {
	tests := []string{
		`name:"unterminated`,
		"name:",
		":value",
		"pid>10",
	}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			if _, err := Parse(input); err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}
