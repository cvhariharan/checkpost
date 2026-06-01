package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/cvhariharan/watcher/internal/alerts"
	"github.com/cvhariharan/watcher/internal/repo"
	"github.com/google/uuid"
	"github.com/invopop/jsonschema"
)

var sourceSchemaReflector = &jsonschema.Reflector{ExpandedStruct: true, DoNotReference: true}

var allowedAlertPlatforms = map[string]bool{"darwin": true, "linux": true, "windows": true}

func RegisterAlertSources(store repo.Store, staleAfter time.Duration) {
	alerts.RegisterSource(policyFailureSource{store: store, staleAfter: staleAfter})
	alerts.RegisterSource(machineOfflineSource{store: store})
}

func validatePlatforms(platforms []string) error {
	for _, p := range platforms {
		if !allowedAlertPlatforms[p] {
			return fmt.Errorf("invalid platform %q", p)
		}
	}
	return nil
}

func enrichWithOwner(ctx context.Context, store repo.Store, nodeID int64, labels map[string]string) {
	owner, err := store.GetNodeOwnerLabels(ctx, nodeID)
	if err != nil {
		return
	}
	if owner.Email != "" {
		labels["owner_email"] = owner.Email
	}
	if owner.ExternalID != "" {
		labels["owner_external_id"] = owner.ExternalID
	}
	if owner.DisplayName != "" {
		labels["owner_display_name"] = owner.DisplayName
	}
}

type policyFailureParams struct {
	Policies     []uuid.UUID `json:"policies,omitempty" jsonschema:"title=Policies,description=Empty = all enabled" jsonschema_extras:"x-widget=text-list,x-placeholder=Policy UUID"`
	Groups       []uuid.UUID `json:"groups,omitempty" jsonschema:"title=Machine groups,description=Empty = all" jsonschema_extras:"x-widget=text-list,x-placeholder=Machine group UUID"`
	Platforms    []string    `json:"platforms,omitempty" jsonschema:"title=Platforms,enum=darwin,enum=linux,enum=windows" jsonschema_extras:"x-widget=multiselect"`
	MinFailing   int         `json:"min_failing,omitempty" jsonschema:"title=Minimum failing,default=1"`
	IncludeStale bool        `json:"include_stale,omitempty" jsonschema:"title=Include stale results"`
}

type policyFailureSource struct {
	store      repo.Store
	staleAfter time.Duration
}

func (policyFailureSource) Type() string { return "policy_failure" }

func (policyFailureSource) Schema() any { return sourceSchemaReflector.Reflect(policyFailureParams{}) }

func (policyFailureSource) ValidateParams(raw json.RawMessage) error {
	var p policyFailureParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	if err := validatePlatforms(p.Platforms); err != nil {
		return err
	}
	if p.MinFailing < 0 {
		return errors.New("min_failing must be >= 0")
	}
	return nil
}

func (s policyFailureSource) Evaluate(ctx context.Context, raw json.RawMessage) ([]alerts.Alert, error) {
	var p policyFailureParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	rows, err := s.store.ListFailingPolicyNodes(ctx, repo.ListFailingPolicyNodesParams{
		Policies:     p.Policies,
		Groups:       p.Groups,
		Platforms:    p.Platforms,
		IncludeStale: p.IncludeStale,
		StaleAfter:   postgresInterval(s.staleAfter),
	})
	if err != nil {
		return nil, err
	}

	nodeSet := map[uuid.UUID]struct{}{}
	for _, r := range rows {
		nodeSet[r.NodeUuid] = struct{}{}
	}
	min := p.MinFailing
	if min < 1 {
		min = 1
	}
	if len(nodeSet) < min {
		return nil, nil
	}

	out := make([]alerts.Alert, 0, len(rows))
	for _, r := range rows {
		labels := map[string]string{
			"host":     r.NodeUuid.String(),
			"hostname": r.Hostname,
			"policy":   r.PolicyName,
			"platform": r.Platform,
		}
		enrichWithOwner(ctx, s.store, r.NodeID, labels)
		out = append(out, alerts.Alert{
			Key:    fmt.Sprintf("policy:%s:node:%s", r.PolicyUuid, r.NodeUuid),
			Labels: labels,
			Annotations: map[string]string{
				"summary":    fmt.Sprintf("%s fails policy %q", r.Hostname, r.PolicyName),
				"resolution": r.Resolution,
			},
		})
	}
	return out, nil
}

type machineOfflineParams struct {
	Threshold  string      `json:"threshold,omitempty" jsonschema:"title=Threshold,description=Go duration; default 24h"`
	Groups     []uuid.UUID `json:"groups,omitempty" jsonschema:"title=Machine groups,description=Empty = all" jsonschema_extras:"x-widget=text-list,x-placeholder=Machine group UUID"`
	Platforms  []string    `json:"platforms,omitempty" jsonschema:"title=Platforms,enum=darwin,enum=linux,enum=windows" jsonschema_extras:"x-widget=multiselect"`
	MinOffline int         `json:"min_offline,omitempty" jsonschema:"title=Minimum offline,default=1"`
}

func (p machineOfflineParams) thresholdDuration() (time.Duration, error) {
	if p.Threshold == "" {
		return 24 * time.Hour, nil
	}
	return time.ParseDuration(p.Threshold)
}

type machineOfflineSource struct {
	store repo.Store
}

func (machineOfflineSource) Type() string { return "machine_offline" }

func (machineOfflineSource) Schema() any {
	return sourceSchemaReflector.Reflect(machineOfflineParams{})
}

func (machineOfflineSource) ValidateParams(raw json.RawMessage) error {
	var p machineOfflineParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	if err := validatePlatforms(p.Platforms); err != nil {
		return err
	}
	d, err := p.thresholdDuration()
	if err != nil {
		return fmt.Errorf("invalid threshold: %w", err)
	}
	if d < time.Minute {
		return errors.New("threshold must be >= 1m")
	}
	if p.MinOffline < 0 {
		return errors.New("min_offline must be >= 0")
	}
	return nil
}

func (s machineOfflineSource) Evaluate(ctx context.Context, raw json.RawMessage) ([]alerts.Alert, error) {
	var p machineOfflineParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	d, err := p.thresholdDuration()
	if err != nil {
		return nil, err
	}
	rows, err := s.store.ListOfflineNodes(ctx, repo.ListOfflineNodesParams{
		Threshold: postgresInterval(d),
		Groups:    p.Groups,
		Platforms: p.Platforms,
	})
	if err != nil {
		return nil, err
	}
	min := p.MinOffline
	if min < 1 {
		min = 1
	}
	if len(rows) < min {
		return nil, nil
	}

	out := make([]alerts.Alert, 0, len(rows))
	for _, r := range rows {
		lastSeen := "never"
		if r.LastSeenAt.Valid {
			lastSeen = r.LastSeenAt.Time.UTC().Format(time.RFC3339)
		}
		labels := map[string]string{
			"host":      r.NodeUuid.String(),
			"hostname":  r.Hostname,
			"platform":  r.Platform,
			"last_seen": lastSeen,
		}
		enrichWithOwner(ctx, s.store, r.NodeID, labels)
		out = append(out, alerts.Alert{
			Key:    fmt.Sprintf("offline:node:%s", r.NodeUuid),
			Labels: labels,
			Annotations: map[string]string{
				"summary": fmt.Sprintf("%s offline since %s", r.Hostname, lastSeen),
			},
		})
	}
	return out, nil
}
