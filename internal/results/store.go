package results

import (
	"context"
	"fmt"

	"github.com/cvhariharan/watcher/internal/repo"
	"github.com/google/uuid"
)

// QueryStore adapts a repo.Querier to the SchemaStore interface used by the
// Writer. It owns no state; the caller threads through a Queries handle.
type QueryStore struct {
	q repo.Querier
}

// NewQueryStore returns a SchemaStore backed by the given Querier (which can
// be a *repo.Queries or any other Querier implementation, including a
// transaction-bound one).
func NewQueryStore(q repo.Querier) *QueryStore {
	return &QueryStore{q: q}
}

// MergeColumns issues the SQL-side union: the database accepts the observed
// list and atomically appends only columns not already present, returning
// the final authoritative order. Concurrent calls for the same partition
// key are serialised by the row lock that ON CONFLICT DO UPDATE takes, so
// no column can be silently dropped.
func (s *QueryStore) MergeColumns(ctx context.Context, scheduleUUID uuid.UUID, sqlVersion int32, observed []string) ([]string, error) {
	raw, err := marshalColumns(observed)
	if err != nil {
		return nil, fmt.Errorf("encode columns: %w", err)
	}
	schema, err := s.q.UpsertQuerySchema(ctx, repo.UpsertQuerySchemaParams{
		ScheduleUuid: scheduleUUID,
		SqlVersion:   sqlVersion,
		Columns:      raw,
	})
	if err != nil {
		return nil, fmt.Errorf("merge query_schemas: %w", err)
	}
	cols, err := unmarshalColumns(schema.Columns)
	if err != nil {
		return nil, fmt.Errorf("decode columns: %w", err)
	}
	return cols, nil
}
