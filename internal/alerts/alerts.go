package alerts

import (
	"context"
	"encoding/json"
	"sync"
)

type EventKind string

const (
	EventFiring   EventKind = "alert.firing"
	EventResolved EventKind = "alert.resolved"
)

type Alert struct {
	Key         string            `json:"key"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Machine map[string]string `json:"machine,omitempty"`
}

func (a Alert) Host() string { return a.Labels["host"] }

// groupByHost buckets alerts by their host label, falling back to a shared key
// for alerts without a host so they are still delivered together.
func groupByHost(alerts []Alert, fallback string) map[string][]Alert {
	groups := map[string][]Alert{}
	for _, a := range alerts {
		host := a.Host()
		if host == "" {
			host = fallback
		}
		groups[host] = append(groups[host], a)
	}
	return groups
}

type Rule struct {
	UUID        string
	Name        string
	Description string
	Source      string
	Severity    string
	Params      json.RawMessage
}

type Target struct {
	UUID   string
	Name   string
	Type   string
	Config json.RawMessage
}

// Source: WHAT to alert on
type Source interface {
	Type() string
	Schema() any
	ValidateParams(params json.RawMessage) error
	Evaluate(ctx context.Context, params json.RawMessage) ([]Alert, error)
}

// Notifier: WHERE alerts go
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

func Sources() []Source {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]Source, 0, len(sources))
	for _, s := range sources {
		out = append(out, s)
	}
	return out
}
