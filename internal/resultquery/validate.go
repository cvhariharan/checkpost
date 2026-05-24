package resultquery

import (
	"fmt"
	"time"
)

// Reserved fields that have special semantics beyond a normal column filter.
const (
	FieldMachine  = "machine"
	FieldLastSeen = "last_seen"
)

// Validate checks each term in filters against the given column set.
// last_seen requires a >= / <= RFC3339 timestamp; machine and column
// filters require : or =. Bare-word terms require at least one column.
// Errors describe the offending term and are safe to surface to clients.
func Validate(filters []Term, columns []string) error {
	if len(filters) == 0 {
		return nil
	}
	known := make(map[string]struct{}, len(columns))
	for _, c := range columns {
		known[c] = struct{}{}
	}

	for _, f := range filters {
		switch f.Field {
		case "":
			if len(columns) == 0 {
				return fmt.Errorf("no searchable columns recorded for this schedule yet")
			}
		case FieldLastSeen:
			if f.Op != OpGTE && f.Op != OpLTE {
				return fmt.Errorf("last_seen only supports >= and <=")
			}
			if _, err := time.Parse(time.RFC3339, f.Value); err != nil {
				return fmt.Errorf("invalid last_seen timestamp")
			}
		case FieldMachine:
			if f.Op != OpContains && f.Op != OpEqual {
				return fmt.Errorf("machine only supports : and =")
			}
		default:
			if _, ok := known[f.Field]; !ok {
				return fmt.Errorf("unknown field %q", f.Field)
			}
			if f.Op != OpContains && f.Op != OpEqual {
				return fmt.Errorf("%s only supports : and =", f.Field)
			}
		}
	}
	return nil
}
