package parquet

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/cvhariharan/checkpost/internal/results"
	"github.com/google/uuid"
)

func TestExportSnapshotCSV(t *testing.T) {
	root := t.TempDir()
	w, err := NewWriter(root, fakeSchemaStore{}, discardLogger())
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	t.Cleanup(func() { w.Close() })

	src := uuid.New()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := w.Submit(src, adHocV, results.KindAdhoc, []results.Row{
		{NodeID: 7, UnixTime: time.Unix(1, 0), Action: "snapshot", Columns: map[string]string{"name": "old"}},
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if err := w.Flush(ctx, src); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	time.Sleep(time.Millisecond)
	if err := w.Submit(src, adHocV, results.KindAdhoc, []results.Row{
		{NodeID: 7, UnixTime: time.Unix(2, 0), Action: "snapshot", Columns: map[string]string{"name": "new"}},
	}); err != nil {
		t.Fatalf("Submit second snapshot: %v", err)
	}
	if err := w.Flush(ctx, src); err != nil {
		t.Fatalf("Flush second snapshot: %v", err)
	}

	r, err := NewReader(root, "")
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	t.Cleanup(func() { r.Close() })

	var out bytes.Buffer
	err = r.Export(ctx, &out, results.ExportRequest{
		Format:   "csv",
		Snapshot: true,
		Columns:  []string{"name"},
		Sources: []results.ExportSource{{
			SourceUUID: src,
			SQLVersion: adHocV,
			NodeID:     7,
		}},
	})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if got, want := out.String(), "name\nnew\n"; got != want {
		t.Fatalf("export CSV = %q, want %q", got, want)
	}

	out.Reset()
	err = r.Export(ctx, &out, results.ExportRequest{
		Format:   "csv",
		Snapshot: true,
		Columns:  []string{"name"},
		Sources: []results.ExportSource{{
			SourceUUID: src,
			SQLVersion: adHocV,
			NodeID:     7,
			Hostname:   "web-01",
		}},
		IncludeHost: true,
	})
	if err != nil {
		t.Fatalf("Export with host: %v", err)
	}
	if got, want := out.String(), "hostname,name\nweb-01,new\n"; got != want {
		t.Fatalf("export CSV with host = %q, want %q", got, want)
	}

	out.Reset()
	err = r.Export(ctx, &out, results.ExportRequest{
		Format:         "csv",
		Snapshot:       true,
		Columns:        []string{"name"},
		Sources:        []results.ExportSource{{SourceUUID: src, SQLVersion: adHocV}},
		Hostnames:      []results.ExportSource{{NodeID: 7, Hostname: "web-01"}},
		IncludeMachine: true,
	})
	if err != nil {
		t.Fatalf("Export with hostname: %v", err)
	}
	if got, want := out.String(), "hostname,last_seen,name\nweb-01,1970-01-01 00:00:02+00,new\n"; got != want {
		t.Fatalf("export CSV with hostname = %q, want %q", got, want)
	}
}
