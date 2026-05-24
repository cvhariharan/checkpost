package resultquery

import (
	"fmt"
	"strings"
)

type Operator string

const (
	OpContains Operator = ":"
	OpEqual    Operator = "="
	OpGTE      Operator = ">="
	OpLTE      Operator = "<="
)

type Term struct {
	Field string
	Op    Operator
	Value string
}

func Parse(input string) ([]Term, error) {
	tokens, err := splitTerms(input)
	if err != nil {
		return nil, err
	}
	terms := make([]Term, 0, len(tokens))
	for _, token := range tokens {
		term, err := parseTerm(token)
		if err != nil {
			return nil, err
		}
		terms = append(terms, term)
	}
	return terms, nil
}

func splitTerms(input string) ([]string, error) {
	var terms []string
	var b strings.Builder
	inQuote := false
	escaped := false

	for _, r := range strings.TrimSpace(input) {
		switch {
		case escaped:
			b.WriteRune(r)
			escaped = false
		case r == '\\' && inQuote:
			escaped = true
		case r == '"':
			inQuote = !inQuote
		case (r == ' ' || r == '\t' || r == '\n' || r == '\r') && !inQuote:
			if b.Len() > 0 {
				terms = append(terms, b.String())
				b.Reset()
			}
		default:
			b.WriteRune(r)
		}
	}

	if escaped {
		return nil, fmt.Errorf("unterminated escape sequence")
	}
	if inQuote {
		return nil, fmt.Errorf("unterminated quoted value")
	}
	if b.Len() > 0 {
		terms = append(terms, b.String())
	}
	return terms, nil
}

func parseTerm(token string) (Term, error) {
	field, op, value, ok := splitOperator(token)
	if !ok {
		if strings.ContainsAny(token, "<>") {
			return Term{}, fmt.Errorf("unsupported operator in %q", token)
		}
		return Term{Op: OpContains, Value: token}, nil
	}
	field = strings.TrimSpace(field)
	value = strings.TrimSpace(value)
	if field == "" {
		return Term{}, fmt.Errorf("field cannot be empty")
	}
	if value == "" {
		return Term{}, fmt.Errorf("value cannot be empty for %s", field)
	}
	if strings.ContainsAny(value, "<>") && op != OpGTE && op != OpLTE {
		return Term{}, fmt.Errorf("unsupported operator in %q", token)
	}
	return Term{Field: field, Op: op, Value: value}, nil
}

func splitOperator(token string) (string, Operator, string, bool) {
	for i := 0; i < len(token); i++ {
		switch token[i] {
		case '>', '<':
			if i+1 < len(token) && token[i+1] == '=' {
				return token[:i], Operator(token[i : i+2]), token[i+2:], true
			}
			return "", "", "", false
		case '=', ':':
			return token[:i], Operator(token[i : i+1]), token[i+1:], true
		}
	}
	return "", "", "", false
}
