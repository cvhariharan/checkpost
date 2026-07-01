package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/cvhariharan/checkpost/internal/alerts"
	"github.com/cvhariharan/checkpost/internal/repo"
	"github.com/invopop/jsonschema"
)

var sourceSchemaReflector = &jsonschema.Reflector{ExpandedStruct: true, DoNotReference: true}

func RegisterAlertSources(store repo.Store, staleAfter time.Duration) {
	alerts.RegisterSource(policyFailureSource{store: store, staleAfter: staleAfter})
	alerts.RegisterSource(machineOfflineSource{store: store})
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

type policyFailureParams struct{}

type policyFailureSource struct {
	store      repo.Store
	staleAfter time.Duration
}

func (policyFailureSource) Type() string { return "policy_failure" }

func (policyFailureSource) Schema() any { return sourceSchemaReflector.Reflect(policyFailureParams{}) }

func (policyFailureSource) ValidateParams(raw json.RawMessage) error { return nil }

func (s policyFailureSource) Evaluate(ctx context.Context, raw json.RawMessage) ([]alerts.Alert, error) {
	rows, err := s.store.ListFailingPolicyNodes(ctx, repo.ListFailingPolicyNodesParams{
		StaleAfter: postgresInterval(s.staleAfter),
	})
	if err != nil {
		return nil, err
	}

	out := make([]alerts.Alert, 0, len(rows))
	for _, r := range rows {
		labels := map[string]string{
			"host":     r.NodeUuid.String(),
			"hostname": r.Hostname,
			"policy":   r.PolicyName,
			"platform": r.Platform,
			"groups":   r.Groups,
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
	Threshold string `json:"threshold,omitempty" jsonschema:"title=Threshold,description=Default 24h"`
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
	d, err := p.thresholdDuration()
	if err != nil {
		return fmt.Errorf("invalid threshold: %w", err)
	}
	if d < time.Minute {
		return errors.New("threshold must be >= 1m")
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
	})
	if err != nil {
		return nil, err
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
			"groups":    r.Groups,
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
