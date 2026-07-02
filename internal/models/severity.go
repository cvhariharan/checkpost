package models

import "strings"

// PolicySeverities is the canonical, ordered (most-severe first) list of policy
// severity levels. It is the single source of truth for the Go runtime; the DB
// CHECK constraint, the request-validation `oneof` tags, and the frontend
// severity options must mirror it.
var PolicySeverities = []string{"critical", "high", "medium", "low", "info"}

// DefaultPolicySeverity is used when a severity is empty or unrecognized. It
// matches the DB column default.
const DefaultPolicySeverity = "medium"

var policySeveritySet = func() map[string]bool {
	m := make(map[string]bool, len(PolicySeverities))
	for _, s := range PolicySeverities {
		m[s] = true
	}
	return m
}()

// IsValidPolicySeverity reports whether s is a recognized severity level.
func IsValidPolicySeverity(s string) bool {
	return policySeveritySet[s]
}

// NormalizePolicySeverity lowercases/trims the severity and falls back to
// DefaultPolicySeverity for empty or unrecognized values.
func NormalizePolicySeverity(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if policySeveritySet[s] {
		return s
	}
	return DefaultPolicySeverity
}
