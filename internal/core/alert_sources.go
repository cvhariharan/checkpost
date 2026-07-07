package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/cvhariharan/checkpost/internal/alerts"
	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/cvhariharan/checkpost/internal/repo"
	"github.com/google/uuid"
	"github.com/invopop/jsonschema"
)

var sourceSchemaReflector = &jsonschema.Reflector{ExpandedStruct: true, DoNotReference: true}

func RegisterAlertSources(store repo.Store, staleAfter time.Duration) {
	alerts.RegisterSource(policyFailureSource{store: store, staleAfter: staleAfter})
	alerts.RegisterSource(machineOfflineSource{store: store, staleAfter: staleAfter})
}

// enrichMachine adds device details (serial, OS, compliance score) to the alert's machine map
func enrichMachine(ctx context.Context, store repo.Store, nodeUUID uuid.UUID, staleAfter time.Duration, machine map[string]string) {
	if n, err := store.GetNodeByUUID(ctx, nodeUUID); err == nil {
		if n.HardwareSerial != "" {
			machine["serial"] = n.HardwareSerial
		}
		if n.OsName != "" {
			machine["os_name"] = n.OsName
		}
		if n.OsVersion != "" {
			machine["os_version"] = n.OsVersion
		}
		if n.OsqueryVersion != "" {
			machine["osquery_version"] = n.OsqueryVersion
		}
	}
	if score, err := store.GetNodeComplianceScore(ctx, repo.GetNodeComplianceScoreParams{
		NodeUuid:    nodeUUID,
		StaleCutoff: time.Now().UTC().Add(-staleAfter),
	}); err == nil {
		if s := weightedComplianceScore(score.WeightedPassing, score.WeightedTotal); s != nil {
			machine["compliance_score"] = strconv.Itoa(*s)
		}
	}
}

func validateSeverities(severities []string) error {
	for _, s := range severities {
		if !models.IsValidPolicySeverity(s) {
			return fmt.Errorf("invalid severity %q", s)
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
	Policies   []uuid.UUID `json:"policies,omitempty" jsonschema:"title=Policies,description=Empty = all enabled policies" jsonschema_extras:"x-widget=resource-select,x-resource=policies"`
	Groups     []uuid.UUID `json:"groups,omitempty" jsonschema:"title=Machine groups,description=Empty = all groups" jsonschema_extras:"x-widget=resource-select,x-resource=groups"`
	Severities []string    `json:"severities,omitempty" jsonschema:"title=Policy severities,description=Empty = all severities,enum=critical,enum=high,enum=medium,enum=low,enum=info" jsonschema_extras:"x-widget=multiselect"`
}

type policyFailureSource struct {
	store      repo.Store
	staleAfter time.Duration
}

func (policyFailureSource) Type() string { return "policy_failure" }

func (policyFailureSource) Schema() any { return sourceSchemaReflector.Reflect(policyFailureParams{}) }

func (policyFailureSource) ValidateParams(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var p policyFailureParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	return validateSeverities(p.Severities)
}

func (s policyFailureSource) Evaluate(ctx context.Context, raw json.RawMessage) ([]alerts.Alert, error) {
	var p policyFailureParams
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
	}
	rows, err := s.store.ListFailingPolicyNodes(ctx, repo.ListFailingPolicyNodesParams{
		Policies:   p.Policies,
		Groups:     p.Groups,
		Severities: p.Severities,
		StaleAfter: postgresInterval(s.staleAfter),
	})
	if err != nil {
		return nil, err
	}

	out := make([]alerts.Alert, 0, len(rows))
	for _, r := range rows {
		labels := map[string]string{
			"host":            r.NodeUuid.String(),
			"hostname":        r.Hostname,
			"policy":          r.PolicyName,
			"policy_severity": r.Severity,
			"platform":        r.Platform,
			"groups":          r.Groups,
		}
		enrichWithOwner(ctx, s.store, r.NodeID, labels)
		machine := map[string]string{
			"hostname": r.Hostname,
			"platform": r.Platform,
		}
		enrichMachine(ctx, s.store, r.NodeUuid, s.staleAfter, machine)
		out = append(out, alerts.Alert{
			Key:     fmt.Sprintf("policy:%s:node:%s", r.PolicyUuid, r.NodeUuid),
			Labels:  labels,
			Machine: machine,
			Annotations: map[string]string{
				"summary":    fmt.Sprintf("%s fails policy %q", r.Hostname, r.PolicyName),
				"resolution": r.Resolution,
			},
		})
	}
	return out, nil
}

type machineOfflineParams struct {
	Threshold string      `json:"threshold,omitempty" jsonschema:"title=Threshold,description=Default 24h"`
	Groups    []uuid.UUID `json:"groups,omitempty" jsonschema:"title=Machine groups,description=Empty = all groups" jsonschema_extras:"x-widget=resource-select,x-resource=groups"`
}

func (p machineOfflineParams) thresholdDuration() (time.Duration, error) {
	if p.Threshold == "" {
		return 24 * time.Hour, nil
	}
	return time.ParseDuration(p.Threshold)
}

type machineOfflineSource struct {
	store      repo.Store
	staleAfter time.Duration
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
		Groups:    p.Groups,
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
		machine := map[string]string{
			"hostname": r.Hostname,
			"platform": r.Platform,
		}
		enrichMachine(ctx, s.store, r.NodeUuid, s.staleAfter, machine)
		out = append(out, alerts.Alert{
			Key:     fmt.Sprintf("offline:node:%s", r.NodeUuid),
			Labels:  labels,
			Machine: machine,
			Annotations: map[string]string{
				"summary": fmt.Sprintf("%s offline since %s", r.Hostname, lastSeen),
			},
		})
	}
	return out, nil
}
