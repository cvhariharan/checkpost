package results

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// memStore is an in-memory SchemaStore for tests. It mirrors the SQL-side
// merge: existing columns keep their order; new observed columns are
// appended only if not already present.
type memStore struct {
	mu   sync.Mutex
	data map[string][]string
}

func newMemStore() *memStore { return &memStore{data: make(map[string][]string)} }

func (m *memStore) key(u uuid.UUID, v int32) string {
	return fmt.Sprintf("%s/%d", u.String(), v)
}

func (m *memStore) MergeColumns(ctx context.Context, u uuid.UUID, v int32, observed []string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := m.key(u, v)
	existing := m.data[k]
	present := make(map[string]struct{}, len(existing))
	for _, c := range existing {
		present[c] = struct{}{}
	}
	out := append([]string{}, existing...)
	for _, c := range observed {
		if _, ok := present[c]; ok {
			continue
		}
		present[c] = struct{}{}
		out = append(out, c)
	}
	m.data[k] = out
	return append([]string{}, out...), nil
}

// columns is a test-only helper that returns the current persisted column
// list without mutating it.
func (m *memStore) columns(u uuid.UUID, v int32) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.data[m.key(u, v)]...)
}

func TestWriterFlushesParquetChunk(t *testing.T) {
	dir := t.TempDir()
	store := newMemStore()
	w, err := NewWriter(dir, store, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}

	u := uuid.New()
	row := Row{
		NodeID:       42,
		UnixTime:     time.Unix(1716471234, 0).UTC(),
		CalendarTime: "Tue Nov 5 12:00:00 2024 UTC",
		Action:       "snapshot",
		RowHash:      []byte("hash-1"),
		Columns:      map[string]string{"pid": "123", "name": "sshd"},
	}
	if err := w.Submit(u, 1, []Row{row}); err != nil {
		t.Fatalf("submit: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	pattern := filepath.Join(dir, "query="+u.String(), "v=1", "host=42", "chunk-*.parquet")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one chunk, got %d (%v)", len(matches), matches)
	}

	info, err := os.Stat(matches[0])
	if err != nil {
		t.Fatalf("stat chunk: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("chunk is empty")
	}

	cols := store.columns(u, 1)
	if len(cols) != 2 {
		t.Fatalf("expected 2 columns, got %v", cols)
	}
}

func TestMergeColumnsConcurrentAddsAllNames(t *testing.T) {
	// memStore's MergeColumns mirrors the SQL contract exercised in
	// internal/repo/queries/query_schemas.sql: concurrent merges must be
	// strictly cumulative. Drives the test mostly as documentation of the
	// expected guarantee — the real serialisation happens in PostgreSQL.
	store := newMemStore()
	u := uuid.New()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		col := "col_" + uuid.New().String()[:8]
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.MergeColumns(context.Background(), u, 1, []string{col}); err != nil {
				t.Errorf("merge: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := len(store.columns(u, 1)); got != 20 {
		t.Fatalf("expected 20 columns after concurrent merges, got %d", got)
	}
}

func TestWriterReaderRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := newMemStore()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	w, err := NewWriter(dir, store, logger)
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}

	u := uuid.New()
	rows := []Row{
		{
			NodeID:       7,
			UnixTime:     time.Now().UTC(),
			CalendarTime: "now",
			Action:       "snapshot",
			RowHash:      []byte("h1"),
			Columns:      map[string]string{"pid": "1", "name": "init"},
		},
		{
			NodeID:       7,
			UnixTime:     time.Now().UTC(),
			CalendarTime: "now",
			Action:       "snapshot",
			RowHash:      []byte("h2"),
			Columns:      map[string]string{"pid": "2", "name": "kthreadd"},
		},
	}
	if err := w.Submit(u, 1, rows); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	r, err := NewReader(dir, "")
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}
	defer r.Close()

	cols := store.columns(u, 1)

	res, err := r.Read(context.Background(), u, 1, cols, ReadOptions{
		NodeID:   7,
		Snapshot: true,
		Limit:    100,
	})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(res.Rows))
	}
	names := make(map[string]struct{})
	for _, row := range res.Rows {
		names[row.Values["name"]] = struct{}{}
	}
	if _, ok := names["init"]; !ok {
		t.Errorf("missing init row: %+v", res.Rows)
	}
	if _, ok := names["kthreadd"]; !ok {
		t.Errorf("missing kthreadd row: %+v", res.Rows)
	}
}

func TestReaderSnapshotKeepsLatestRowsPerHost(t *testing.T) {
	dir := t.TempDir()
	store := newMemStore()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	u := uuid.New()

	writeRows := func(rows []Row) {
		t.Helper()
		w, err := NewWriter(dir, store, logger)
		if err != nil {
			t.Fatalf("new writer: %v", err)
		}
		if err := w.Submit(u, 1, rows); err != nil {
			t.Fatalf("submit: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	}

	writeRows([]Row{
		{
			NodeID:       7,
			UnixTime:     time.Unix(100, 0).UTC(),
			CalendarTime: "old",
			Action:       "snapshot",
			RowHash:      []byte("host-7-old"),
			Columns:      map[string]string{"name": "old-host-7"},
		},
		{
			NodeID:       8,
			UnixTime:     time.Unix(100, 0).UTC(),
			CalendarTime: "only",
			Action:       "snapshot",
			RowHash:      []byte("host-8-only"),
			Columns:      map[string]string{"name": "only-host-8"},
		},
	})
	time.Sleep(time.Millisecond)
	writeRows([]Row{
		{
			NodeID:       7,
			UnixTime:     time.Unix(200, 0).UTC(),
			CalendarTime: "new",
			Action:       "snapshot",
			RowHash:      []byte("host-7-new"),
			Columns:      map[string]string{"name": "new-host-7"},
		},
	})

	r, err := NewReader(dir, "")
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}
	defer r.Close()

	res, err := r.Read(context.Background(), u, 1, store.columns(u, 1), ReadOptions{
		Snapshot: true,
		Limit:    100,
	})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if res.Total != 2 || len(res.Rows) != 2 {
		t.Fatalf("expected 2 latest snapshot rows, got total=%d rows=%d: %+v", res.Total, len(res.Rows), res.Rows)
	}

	names := make(map[string]struct{}, len(res.Rows))
	for _, row := range res.Rows {
		names[row.Values["name"]] = struct{}{}
	}
	for _, want := range []string{"new-host-7", "only-host-8"} {
		if _, ok := names[want]; !ok {
			t.Fatalf("missing %s in %+v", want, res.Rows)
		}
	}
	if _, ok := names["old-host-7"]; ok {
		t.Fatalf("old host 7 snapshot was not replaced: %+v", res.Rows)
	}
}

func TestReaderDifferentialKeepsSameHashPerHost(t *testing.T) {
	dir := t.TempDir()
	store := newMemStore()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	w, err := NewWriter(dir, store, logger)
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}

	u := uuid.New()
	rows := []Row{
		{
			NodeID:       7,
			UnixTime:     time.Unix(100, 0).UTC(),
			CalendarTime: "now",
			Action:       "added",
			RowHash:      []byte("same-hash"),
			Columns:      map[string]string{"name": "shared"},
		},
		{
			NodeID:       8,
			UnixTime:     time.Unix(100, 0).UTC(),
			CalendarTime: "now",
			Action:       "added",
			RowHash:      []byte("same-hash"),
			Columns:      map[string]string{"name": "shared"},
		},
	}
	if err := w.Submit(u, 1, rows); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	r, err := NewReader(dir, "")
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}
	defer r.Close()

	res, err := r.Read(context.Background(), u, 1, store.columns(u, 1), ReadOptions{Limit: 100})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if res.Total != 2 || len(res.Rows) != 2 {
		t.Fatalf("expected same row hash from two hosts to survive, got total=%d rows=%d: %+v", res.Total, len(res.Rows), res.Rows)
	}
}
