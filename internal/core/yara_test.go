package core

import (
	"strings"
	"testing"
)

func TestYaraFileQueryIncludesEveryPathForEachRuleURL(t *testing.T) {
	query := yaraFileQuery(
		[]string{"/tmp/%", "/Users/%/Downloads/%"},
		[]string{"https://rules.example.com/a.yar", "https://rules.example.com/b.yar"},
	)

	expectedPathClause := "(path LIKE '/tmp/%' OR path LIKE '/Users/%/Downloads/%')"
	if got := strings.Count(query, expectedPathClause); got != 2 {
		t.Fatalf("expected path clause once per rule URL, got %d in %q", got, query)
	}
	for _, ruleURL := range []string{"https://rules.example.com/a.yar", "https://rules.example.com/b.yar"} {
		if !strings.Contains(query, "sigurl = '"+ruleURL+"'") {
			t.Fatalf("expected query to include rule URL %q: %s", ruleURL, query)
		}
	}
	if got := strings.Count(query, " UNION ALL "); got != 1 {
		t.Fatalf("expected one UNION ALL between rule URL queries, got %d in %q", got, query)
	}
}

func TestYaraFileQueryEscapesSingleQuotes(t *testing.T) {
	query := yaraFileQuery([]string{"/tmp/it's/%"}, []string{"https://rules.example.com/it's.yar"})

	if !strings.Contains(query, "path LIKE '/tmp/it''s/%'") {
		t.Fatalf("expected escaped path in %q", query)
	}
	if !strings.Contains(query, "sigurl = 'https://rules.example.com/it''s.yar'") {
		t.Fatalf("expected escaped rule URL in %q", query)
	}
}
