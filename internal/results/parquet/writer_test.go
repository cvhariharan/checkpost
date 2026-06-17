package parquet

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/cvhariharan/checkpost/internal/results"
	"github.com/google/uuid"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeSchemaStore echoes the observed columns back, sorted, so the writer can
// resolve a schema without a database.
type fakeSchemaStore struct{}

func (fakeSchemaStore) MergeColumns(_ context.Context, _ uuid.UUID, _ int32, observed []string, _ string) ([]string, error) {
	cols := append([]string(nil), observed...)
	sort.Strings(cols)
	return cols, nil
}

// TestFlushMakesRowsReadableImmediately asserts that Flush forces buffered rows
// to a chunk on disk synchronously — well before the 30s flush timer — and
// returns without hanging.
func TestFlushMakesRowsReadableImmediately(t *testing.T) {
	root := t.TempDir()
	w, err := NewWriter(root, fakeSchemaStore{}, discardLogger())
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	t.Cleanup(func() { w.Close() })

	src := uuid.New()
	rows := []results.Row{
		{NodeID: 7, UnixTime: time.Unix(1, 0), Action: "snapshot", Columns: map[string]string{"name": "a"}},
		{NodeID: 7, UnixTime: time.Unix(2, 0), Action: "snapshot", Columns: map[string]string{"name": "b"}},
	}
	if err := w.Submit(src, adHocV, results.KindAdhoc, rows); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := w.Flush(ctx, src); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	key := PartitionKey{SourceUUID: src, SQLVersion: adHocV, NodeID: 7, Kind: results.KindAdhoc}
	chunks, err := filepath.Glob(filepath.Join(partitionDir(root, key), "chunk-*.parquet"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected a parquet chunk on disk immediately after Flush, found none")
	}
}

// TestFlushUnknownSourceNoop ensures flushing a source with no buffered rows
// returns promptly instead of blocking.
func TestFlushUnknownSourceNoop(t *testing.T) {
	w, err := NewWriter(t.TempDir(), fakeSchemaStore{}, discardLogger())
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	t.Cleanup(func() { w.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := w.Flush(ctx, uuid.New()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
}

const adHocV int32 = 1
