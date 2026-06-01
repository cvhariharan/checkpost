// Package alerts is a standalone, registry-based alerting subsystem: Sources
// define what to alert on, Notifiers define where alerts go, and the Engine
// edge-triggers deliveries by diffing successive Source evaluations.
package alerts

import (
	"context"
	"encoding/json"
	"sync"
)

// EventKind is the transition that triggered a delivery.
type EventKind string

const (
	EventFiring   EventKind = "alert.firing"
	EventResolved EventKind = "alert.resolved"
)

// Alert is a normalized match. Key is the stable identity used for edge
// detection; the "host" label groups deliveries.
type Alert struct {
	Key         string            `json:"key"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// Host returns the delivery-grouping key (the "host" label, or "" if absent).
func (a Alert) Host() string { return a.Labels["host"] }

// Rule is the engine-resolved rule handed to notifiers (read-only view).
type Rule struct {
	UUID        string
	Name        string
	Description string
	Source      string
	Severity    string
	Params      json.RawMessage
}

// Target is a resolved delivery target.
type Target struct {
	UUID   string
	Name   string
	Type   string
	Config json.RawMessage
}

// Source: WHAT to alert on. Implemented once per feature, registered at startup.
type Source interface {
	Type() string
	Schema() any
	ValidateParams(params json.RawMessage) error
	Evaluate(ctx context.Context, params json.RawMessage) ([]Alert, error)
}

// Notifier: WHERE alerts go. Registered at startup.
type Notifier interface {
	Type() string
	Schema() any
	ValidateConfig(config json.RawMessage) (json.RawMessage, error)
	Send(ctx context.Context, kind EventKind, target Target, alerts []Alert, rule Rule) error
}

var (
	registryMu sync.RWMutex
	sources    = map[string]Source{}
	notifiers  = map[string]Notifier{}
)

func RegisterSource(s Source) {
	registryMu.Lock()
	defer registryMu.Unlock()
	sources[s.Type()] = s
}

func RegisterNotifier(n Notifier) {
	registryMu.Lock()
	defer registryMu.Unlock()
	notifiers[n.Type()] = n
}

func LookupSource(t string) (Source, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	s, ok := sources[t]
	return s, ok
}

func LookupNotifier(t string) (Notifier, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	n, ok := notifiers[t]
	return n, ok
}

// Sources returns the registered source types in a stable-enough map copy.
func Sources() []Source {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]Source, 0, len(sources))
	for _, s := range sources {
		out = append(out, s)
	}
	return out
}
