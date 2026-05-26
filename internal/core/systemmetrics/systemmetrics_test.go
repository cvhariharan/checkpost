package systemmetrics

import (
	"reflect"
	"testing"

	"github.com/cvhariharan/watcher/internal/results"
	"github.com/invopop/jsonschema"
)

type stubExtractor struct {
	kind  Kind
	value any
	ok    bool
}

func (s stubExtractor) Kind() Kind                        { return s.kind }
func (stubExtractor) Schema() *jsonschema.Schema          { return &jsonschema.Schema{} }
func (s stubExtractor) Extract([]results.Row) (any, bool) { return s.value, s.ok }

func row(cols map[string]string) results.Row {
	return results.Row{Columns: cols}
}

func TestRegistryApplyUnknownSchedule(t *testing.T) {
	r := New()
	if _, ok := r.Apply("does-not-exist", nil); ok {
		t.Fatalf("expected ok=false for unknown schedule")
	}
}

func TestRegistryApplyPropagatesExtractor(t *testing.T) {
	r := New()
	r.Register(stubExtractor{kind: KindDisk, value: "/", ok: true}, "Test")

	snap, ok := r.Apply("Test", []results.Row{{Columns: map[string]string{"path": "/"}}})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if snap.Kind != KindDisk {
		t.Fatalf("got kind %q, want %q", snap.Kind, KindDisk)
	}
	if got, ok := snap.Value.(string); !ok || got != "/" {
		t.Fatalf("got value %v, want '/'", snap.Value)
	}
}

func TestRegistryApplySkipsWhenExtractorDeclines(t *testing.T) {
	r := New()
	r.Register(stubExtractor{kind: KindDisk, ok: false}, "Test")
	if _, ok := r.Apply("Test", nil); ok {
		t.Fatalf("expected ok=false when extractor declines")
	}
}

func TestRegistryRegisterOverwrites(t *testing.T) {
	r := New()
	r.Register(stubExtractor{kind: KindDisk, value: "first", ok: true}, "X")
	r.Register(stubExtractor{kind: KindMemory, value: "second", ok: true}, "X")

	snap, ok := r.Apply("X", []results.Row{{}})
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if snap.Kind != KindMemory {
		t.Fatalf("got kind %q, want %q", snap.Kind, KindMemory)
	}
	if snap.Value != "second" {
		t.Fatalf("got value %v, want 'second'", snap.Value)
	}
}

func TestRegistryRegisterFanout(t *testing.T) {
	r := New()
	r.Register(stubExtractor{kind: KindDisk, value: "ok", ok: true}, "A", "B")

	for _, name := range []string{"A", "B"} {
		snap, ok := r.Apply(name, []results.Row{{}})
		if !ok || snap.Kind != KindDisk {
			t.Fatalf("schedule %q: got (%+v, %v), want kind=disk ok=true", name, snap, ok)
		}
	}
}

func TestDefaultCoversEveryKind(t *testing.T) {
	r := Default()
	want := map[Kind][]string{
		KindDisk:    {"Disk Usage POSIX", "Disk Usage Windows"},
		KindNetwork: {"Network Interfaces POSIX", "Network Interfaces Windows"},
		KindMemory:  {"Memory Usage POSIX", "Memory Usage Windows"},
		KindCPU:     {"CPU Info"},
		KindOSInfo:  {"OS Info"},
		KindUptime:  {"System Uptime"},
	}
	for kind, names := range want {
		for _, name := range names {
			e, ok := r.entries[name]
			if !ok {
				t.Errorf("schedule %q not registered", name)
				continue
			}
			if e.Kind() != kind {
				t.Errorf("schedule %q: got kind %q, want %q", name, e.Kind(), kind)
			}
		}
		if r.schemas[kind] == nil {
			t.Errorf("kind %q has no memoized schema", kind)
		}
	}
}

func TestDefaultSchemasReflect(t *testing.T) {
	r := Default()
	for kind, schema := range r.Schemas() {
		if schema == nil {
			t.Errorf("kind %q: nil schema", kind)
			continue
		}
		if schema.Properties == nil || schema.Properties.Len() == 0 {
			t.Errorf("kind %q: schema has no properties", kind)
		}
	}
}

func TestDefaultKindsSorted(t *testing.T) {
	got := Default().Kinds()
	want := []Kind{KindCPU, KindDisk, KindMemory, KindNetwork, KindOSInfo, KindUptime}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestDiskExtractor(t *testing.T) {
	e := DiskExtractor{}
	if e.Kind() != KindDisk {
		t.Fatalf("kind: got %q, want %q", e.Kind(), KindDisk)
	}

	rows := []results.Row{
		row(map[string]string{"path": "/", "type": "apfs", "free_gb": "12.5", "total_gb": "256", "free_perc": "4.88"}),
		row(map[string]string{"path": "", "free_gb": "9"}),
		row(map[string]string{"path": "/data", "type": "ext4", "free_gb": "100", "total_gb": "500", "free_perc": "20"}),
	}
	got, ok := e.Extract(rows)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	want := DiskValue{Mounts: []DiskMount{
		{Path: "/", Type: "apfs", FreeGB: 12.5, TotalGB: 256, FreePct: 4.88},
		{Path: "/data", Type: "ext4", FreeGB: 100, TotalGB: 500, FreePct: 20},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	if _, ok := e.Extract(nil); ok {
		t.Fatalf("expected ok=false for nil rows")
	}
	if _, ok := e.Extract([]results.Row{row(map[string]string{"path": ""})}); ok {
		t.Fatalf("expected ok=false for rows with empty path")
	}
}

func TestNetworkExtractor(t *testing.T) {
	e := NetworkExtractor{}
	if e.Kind() != KindNetwork {
		t.Fatalf("kind: got %q, want %q", e.Kind(), KindNetwork)
	}

	rows := []results.Row{
		row(map[string]string{"name": "eth0", "address": "192.168.1.10", "mac": "aa:bb:cc:dd:ee:ff"}),
		row(map[string]string{"name": "eth0", "address": "fe80::1", "mac": "aa:bb:cc:dd:ee:ff"}),
		row(map[string]string{"name": "eth0", "address": "192.168.1.10", "mac": "aa:bb:cc:dd:ee:ff"}), // dup
		row(map[string]string{"name": "lo", "address": "127.0.0.1"}),
		row(map[string]string{"name": "", "address": "10.0.0.1"}),
	}
	got, ok := e.Extract(rows)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	want := NetworkValue{Interfaces: []NetworkInterface{
		{Name: "eth0", MAC: "aa:bb:cc:dd:ee:ff", Addresses: []string{"192.168.1.10", "fe80::1"}},
		{Name: "lo", Addresses: []string{"127.0.0.1"}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestMemoryExtractor(t *testing.T) {
	e := MemoryExtractor{}
	if e.Kind() != KindMemory {
		t.Fatalf("kind: got %q, want %q", e.Kind(), KindMemory)
	}

	rows := []results.Row{row(map[string]string{"total_bytes": "17179869184", "available_bytes": "4053934080"})}
	got, ok := e.Extract(rows)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	want := MemoryValue{TotalBytes: 17179869184, AvailableBytes: 4053934080}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	if _, ok := e.Extract([]results.Row{row(map[string]string{"total_bytes": "0", "available_bytes": "0"})}); ok {
		t.Fatalf("expected ok=false for all-zero row")
	}
}

func TestCPUExtractor(t *testing.T) {
	e := CPUExtractor{}
	if e.Kind() != KindCPU {
		t.Fatalf("kind: got %q, want %q", e.Kind(), KindCPU)
	}

	rows := []results.Row{row(map[string]string{"model": "Apple M2", "physical_cores": "8", "logical_cores": "8"})}
	got, ok := e.Extract(rows)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	want := CPUValue{Model: "Apple M2", PhysicalCores: 8, LogicalCores: 8}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestOSInfoExtractor(t *testing.T) {
	e := OSInfoExtractor{}
	if e.Kind() != KindOSInfo {
		t.Fatalf("kind: got %q, want %q", e.Kind(), KindOSInfo)
	}

	rows := []results.Row{row(map[string]string{"name": "macOS", "version": "14.5", "build": "23F79", "platform": "darwin"})}
	got, ok := e.Extract(rows)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	want := OSInfoValue{Name: "macOS", Version: "14.5", Build: "23F79", Platform: "darwin"}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestUptimeExtractor(t *testing.T) {
	e := UptimeExtractor{}
	if e.Kind() != KindUptime {
		t.Fatalf("kind: got %q, want %q", e.Kind(), KindUptime)
	}

	rows := []results.Row{row(map[string]string{"seconds": "12345"})}
	got, ok := e.Extract(rows)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if got != (UptimeValue{Seconds: 12345}) {
		t.Fatalf("got %+v, want seconds=12345", got)
	}

	if _, ok := e.Extract([]results.Row{row(map[string]string{"seconds": "0"})}); ok {
		t.Fatalf("expected ok=false for zero seconds")
	}
}
