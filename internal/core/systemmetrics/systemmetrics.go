// Package systemmetrics maps is_system osquery schedules to canonical
// device-metric snapshots.
//
// Adding a new metric:
//
//  1. Define the value type as a struct in extractors.go with json tags
//     and jsonschema / jsonschema_extras struct tags as needed.
//  2. Implement Extractor (Kind + Schema + Extract) — usually a stateless
//     companion struct.
//  3. Bind it to one or more schedule names in Default.
//  4. Seed the matching system query/schedule in the migration.
//
// No frontend change is required: MetricRenderer.svelte consumes the schema.
package systemmetrics

import (
	"sort"

	"github.com/cvhariharan/watcher/internal/results"
	"github.com/invopop/jsonschema"
)

// Kind names a class of device metric. Stable string — written to
// node_metrics.kind and surfaced in the schemas endpoint.
type Kind string

const (
	KindDisk    Kind = "disk"
	KindNetwork Kind = "network"
	KindMemory  Kind = "memory"
	KindCPU     Kind = "cpu"
	KindOSInfo  Kind = "os_info"
	KindUptime  Kind = "uptime"
)

// Snapshot is the reduction of one (host, schedule) result batch.
type Snapshot struct {
	Kind  Kind
	Value any
}

// Extractor is the unit of metric extensibility. Implementations are
// stateless; Schema is reflected once at registry construction.
type Extractor interface {
	Kind() Kind
	Schema() *jsonschema.Schema
	Extract(rows []results.Row) (value any, ok bool)
}

// Registry maps schedule names to Extractors and memoizes each kind's
// reflected schema. Build via New or Default; the zero value is not usable.
type Registry struct {
	entries map[string]Extractor
	schemas map[Kind]*jsonschema.Schema
}

func New() *Registry {
	return &Registry{
		entries: make(map[string]Extractor),
		schemas: make(map[Kind]*jsonschema.Schema),
	}
}

// Register binds each schedule name to e and memoizes e.Schema() for the
// kind. Later registrations for the same name or kind overwrite earlier ones.
func (r *Registry) Register(e Extractor, scheduleNames ...string) {
	r.schemas[e.Kind()] = e.Schema()
	for _, name := range scheduleNames {
		r.entries[name] = e
	}
}

func (r *Registry) Apply(scheduleName string, rows []results.Row) (Snapshot, bool) {
	e, ok := r.entries[scheduleName]
	if !ok {
		return Snapshot{}, false
	}
	value, ok := e.Extract(rows)
	if !ok {
		return Snapshot{}, false
	}
	return Snapshot{Kind: e.Kind(), Value: value}, true
}

// Schemas returns kind → reflected schema. Callers must not mutate.
func (r *Registry) Schemas() map[Kind]*jsonschema.Schema {
	return r.schemas
}

// Kinds returns the registered kinds in alphabetical order — the canonical
// render order surfaced to the frontend.
func (r *Registry) Kinds() []Kind {
	out := make([]Kind, 0, len(r.schemas))
	for k := range r.schemas {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Default returns the registry wired with every built-in system schedule.
// Schedule names must match the ones the migration seeds.
func Default() *Registry {
	r := New()
	r.Register(DiskExtractor{}, "Disk Usage POSIX", "Disk Usage Windows")
	r.Register(NetworkExtractor{}, "Network Interfaces POSIX", "Network Interfaces Windows")
	r.Register(MemoryExtractor{}, "Memory Usage POSIX", "Memory Usage Windows")
	r.Register(CPUExtractor{}, "CPU Info")
	r.Register(OSInfoExtractor{}, "OS Info")
	r.Register(UptimeExtractor{}, "System Uptime")
	return r
}
