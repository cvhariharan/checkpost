package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cvhariharan/checkpost/internal/alerts"
	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/cvhariharan/checkpost/internal/repo"
	"github.com/google/uuid"
)

// ListAlertSources returns the registered source types and their JSON Schemas.
func (c *Core) ListAlertSources() []models.AlertSourceInfo {
	srcs := alerts.Sources()
	out := make([]models.AlertSourceInfo, 0, len(srcs))
	for _, s := range srcs {
		out = append(out, models.AlertSourceInfo{Type: s.Type(), Schema: s.Schema()})
	}
	return out
}

func (c *Core) CreateAlertTarget(ctx context.Context, req models.CreateAlertTarget) (models.AlertTarget, error) {
	notifier, ok := alerts.LookupNotifier(req.Type)
	if !ok {
		return models.AlertTarget{}, fmt.Errorf("unknown notifier type %q", req.Type)
	}
	config, err := notifier.ValidateConfig(req.Config)
	if err != nil {
		return models.AlertTarget{}, fmt.Errorf("invalid target config: %w", err)
	}

	target, err := c.store.CreateAlertTarget(ctx, repo.CreateAlertTargetParams{
		Name:    strings.TrimSpace(req.Name),
		Type:    req.Type,
		Config:  config,
		Enabled: req.Enabled,
	})
	if err != nil {
		return models.AlertTarget{}, fmt.Errorf("create alert target: %w", err)
	}
	return toModelAlertTarget(target), nil
}

func (c *Core) UpdateAlertTarget(ctx context.Context, req models.UpdateAlertTarget) (models.AlertTarget, error) {
	id, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.AlertTarget{}, fmt.Errorf("parse target uuid: %w", err)
	}
	existing, err := c.store.GetAlertTargetByUUID(ctx, id)
	if err != nil {
		return models.AlertTarget{}, fmt.Errorf("get alert target: %w", err)
	}
	notifier, ok := alerts.LookupNotifier(existing.Type)
	if !ok {
		return models.AlertTarget{}, fmt.Errorf("unknown notifier type %q", existing.Type)
	}

	config, err := notifier.ValidateConfig(req.Config)
	if err != nil {
		return models.AlertTarget{}, fmt.Errorf("invalid target config: %w", err)
	}

	target, err := c.store.UpdateAlertTargetByUUID(ctx, repo.UpdateAlertTargetByUUIDParams{
		Uuid:    id,
		Name:    strings.TrimSpace(req.Name),
		Config:  config,
		Enabled: req.Enabled,
	})
	if err != nil {
		return models.AlertTarget{}, fmt.Errorf("update alert target: %w", err)
	}
	return toModelAlertTarget(target), nil
}

func (c *Core) GetAlertTarget(ctx context.Context, req models.ResourceID) (models.AlertTarget, error) {
	id, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.AlertTarget{}, fmt.Errorf("parse target uuid: %w", err)
	}
	target, err := c.store.GetAlertTargetByUUID(ctx, id)
	if err != nil {
		return models.AlertTarget{}, fmt.Errorf("get alert target: %w", err)
	}
	return toModelAlertTarget(target), nil
}

func (c *Core) PaginateAlertTargets(ctx context.Context, req models.PageRequest) (models.Page[models.AlertTarget], error) {
	count, page := pageBounds(req)
	rows, err := c.store.ListAlertTargets(ctx, repo.ListAlertTargetsParams{
		Limit:  int32(count),
		Offset: int32(page * count),
	})
	if err != nil {
		return models.Page[models.AlertTarget]{}, fmt.Errorf("list alert targets: %w", err)
	}
	out := make([]models.AlertTarget, 0, len(rows))
	total := 0
	for _, r := range rows {
		out = append(out, toModelAlertTarget(repo.AlertTarget{
			Uuid: r.Uuid, Name: r.Name, Type: r.Type, Config: r.Config,
			Enabled: r.Enabled, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}))
		total = int(r.TotalCount)
	}
	return models.Page[models.AlertTarget]{Items: out, TotalCount: total, PageCount: pageCountFor(total, count)}, nil
}

func (c *Core) DeleteAlertTarget(ctx context.Context, req models.ResourceID) error {
	id, err := uuid.Parse(req.UUID)
	if err != nil {
		return fmt.Errorf("parse target uuid: %w", err)
	}
	rows, err := c.store.DeleteAlertTargetByUUID(ctx, id)
	if err != nil {
		return fmt.Errorf("delete alert target: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// TestTarget sends a sample firing alert to the target.
func (c *Core) TestTarget(ctx context.Context, req models.ResourceID) error {
	id, err := uuid.Parse(req.UUID)
	if err != nil {
		return fmt.Errorf("parse target uuid: %w", err)
	}
	target, err := c.store.GetAlertTargetByUUID(ctx, id)
	if err != nil {
		return fmt.Errorf("get alert target: %w", err)
	}
	notifier, ok := alerts.LookupNotifier(target.Type)
	if !ok {
		return fmt.Errorf("unknown notifier type %q", target.Type)
	}
	sample := []alerts.Alert{{
		Key:         "test:sample",
		Labels:      map[string]string{"host": "test-host", "hostname": "test-host"},
		Machine:     map[string]string{"hostname": "test-host", "platform": "darwin", "serial": "TESTSERIAL123", "compliance_score": "42"},
		Annotations: map[string]string{"summary": "This is a test alert from Checkpost."},
	}}
	rule := alerts.Rule{Name: "Test alert", Severity: "info", Source: target.Type}
	return notifier.Send(ctx, alerts.EventFiring, alerts.Target{
		UUID: target.Uuid.String(), Name: target.Name, Type: target.Type, Config: target.Config,
	}, sample, rule)
}

func (c *Core) CreateAlertRule(ctx context.Context, req models.CreateAlertRule) (models.AlertRule, error) {
	if err := c.validateRuleSource(req.Source, req.Params); err != nil {
		return models.AlertRule{}, err
	}
	targetUUIDs, err := parseUUIDList(req.TargetIDs, "target")
	if err != nil {
		return models.AlertRule{}, err
	}

	rule, err := c.store.CreateAlertRuleTx(ctx, repo.CreateAlertRuleTxParams{
		Rule: repo.CreateAlertRuleParams{
			Name:                      strings.TrimSpace(req.Name),
			Description:               req.Description,
			Source:                    req.Source,
			Params:                    normalizeParams(req.Params),
			Severity:                  req.Severity,
			Enabled:                   req.Enabled,
			EvaluationIntervalSeconds: int32(req.EvaluationInterval),
			ForSeconds:                int32(req.For),
			RepeatIntervalSeconds:     int32(req.RepeatInterval),
		},
		TargetUUIDs: targetUUIDs,
	})
	if err != nil {
		return models.AlertRule{}, fmt.Errorf("create alert rule: %w", err)
	}
	return c.toModelAlertRule(ctx, rule)
}

func (c *Core) UpdateAlertRule(ctx context.Context, req models.UpdateAlertRule) (models.AlertRule, error) {
	id, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.AlertRule{}, fmt.Errorf("parse rule uuid: %w", err)
	}
	if err := c.validateRuleSource(req.Source, req.Params); err != nil {
		return models.AlertRule{}, err
	}
	targetUUIDs, err := parseUUIDList(req.TargetIDs, "target")
	if err != nil {
		return models.AlertRule{}, err
	}

	rule, err := c.store.UpdateAlertRuleTx(ctx, repo.UpdateAlertRuleTxParams{
		Rule: repo.UpdateAlertRuleByUUIDParams{
			Uuid:                      id,
			Name:                      strings.TrimSpace(req.Name),
			Description:               req.Description,
			Source:                    req.Source,
			Params:                    normalizeParams(req.Params),
			Severity:                  req.Severity,
			Enabled:                   req.Enabled,
			EvaluationIntervalSeconds: int32(req.EvaluationInterval),
			ForSeconds:                int32(req.For),
			RepeatIntervalSeconds:     int32(req.RepeatInterval),
		},
		TargetUUIDs: targetUUIDs,
	})
	if err != nil {
		return models.AlertRule{}, fmt.Errorf("update alert rule: %w", err)
	}
	return c.toModelAlertRule(ctx, rule)
}

func (c *Core) GetAlertRule(ctx context.Context, req models.ResourceID) (models.AlertRule, error) {
	id, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.AlertRule{}, fmt.Errorf("parse rule uuid: %w", err)
	}
	rule, err := c.store.GetAlertRuleByUUID(ctx, id)
	if err != nil {
		return models.AlertRule{}, fmt.Errorf("get alert rule: %w", err)
	}
	return c.toModelAlertRule(ctx, rule)
}

func (c *Core) PaginateAlertRules(ctx context.Context, req models.PageRequest) (models.Page[models.AlertRule], error) {
	count, page := pageBounds(req)
	rows, err := c.store.ListAlertRules(ctx, repo.ListAlertRulesParams{
		Limit:  int32(count),
		Offset: int32(page * count),
	})
	if err != nil {
		return models.Page[models.AlertRule]{}, fmt.Errorf("list alert rules: %w", err)
	}
	out := make([]models.AlertRule, 0, len(rows))
	total := 0
	for _, r := range rows {
		m, err := c.toModelAlertRule(ctx, repo.AlertRule{
			ID: r.ID, Uuid: r.Uuid, Name: r.Name, Description: r.Description, Source: r.Source,
			Params: r.Params, Severity: r.Severity, Enabled: r.Enabled,
			EvaluationIntervalSeconds: r.EvaluationIntervalSeconds, ForSeconds: r.ForSeconds,
			RepeatIntervalSeconds: r.RepeatIntervalSeconds, LastEvaluatedAt: r.LastEvaluatedAt,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		})
		if err != nil {
			return models.Page[models.AlertRule]{}, err
		}
		out = append(out, m)
		total = int(r.TotalCount)
	}
	return models.Page[models.AlertRule]{Items: out, TotalCount: total, PageCount: pageCountFor(total, count)}, nil
}

// PaginateAlertInstances returns the live alert_state instances (firing/pending)
// for a rule, optionally filtered by status.
func (c *Core) PaginateAlertInstances(ctx context.Context, req models.AlertInstancesRequest) (models.Page[models.AlertInstance], error) {
	ruleID, err := uuid.Parse(req.RuleUUID)
	if err != nil {
		return models.Page[models.AlertInstance]{}, fmt.Errorf("parse rule uuid: %w", err)
	}
	rule, err := c.store.GetAlertRuleByUUID(ctx, ruleID)
	if err != nil {
		return models.Page[models.AlertInstance]{}, fmt.Errorf("get alert rule: %w", err)
	}

	count, page := pageBounds(models.PageRequest{Page: req.Page, Count: req.Count})
	status := strings.ToLower(strings.TrimSpace(req.Status))
	switch status {
	case "", "firing", "pending":
	default:
		return models.Page[models.AlertInstance]{}, fmt.Errorf("invalid alert status %q", req.Status)
	}

	rows, err := c.store.ListAlertStateByRulePaginated(ctx, repo.ListAlertStateByRulePaginatedParams{
		RuleID:     rule.ID,
		Status:     status,
		PageCount:  int32(count),
		PageOffset: int32(page * count),
	})
	if err != nil {
		return models.Page[models.AlertInstance]{}, fmt.Errorf("list alert instances: %w", err)
	}

	out := make([]models.AlertInstance, 0, len(rows))
	total := 0
	for _, r := range rows {
		inst := models.AlertInstance{
			AlertKey:    r.AlertKey,
			Status:      r.Status,
			Labels:      r.Labels,
			Annotations: r.Annotations,
			FirstSeenAt: r.FirstSeenAt,
			LastSeenAt:  r.LastSeenAt,
		}
		if r.LastNotifiedAt.Valid {
			t := r.LastNotifiedAt.Time
			inst.LastNotifiedAt = &t
		}
		out = append(out, inst)
		total = int(r.TotalCount)
	}
	return models.Page[models.AlertInstance]{Items: out, TotalCount: total, PageCount: pageCountFor(total, count)}, nil
}

func (c *Core) DeleteAlertRule(ctx context.Context, req models.ResourceID) error {
	id, err := uuid.Parse(req.UUID)
	if err != nil {
		return fmt.Errorf("parse rule uuid: %w", err)
	}
	rows, err := c.store.DeleteAlertRuleByUUID(ctx, id)
	if err != nil {
		return fmt.Errorf("delete alert rule: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (c *Core) validateRuleSource(source string, params json.RawMessage) error {
	src, ok := alerts.LookupSource(source)
	if !ok {
		return fmt.Errorf("unknown alert source %q", source)
	}
	if err := src.ValidateParams(normalizeParams(params)); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}
	return nil
}

func (c *Core) toModelAlertRule(ctx context.Context, r repo.AlertRule) (models.AlertRule, error) {
	targetUUIDs, err := c.store.ListTargetUUIDsForRule(ctx, r.ID)
	if err != nil {
		return models.AlertRule{}, fmt.Errorf("list rule targets: %w", err)
	}
	ids := make([]string, 0, len(targetUUIDs))
	for _, u := range targetUUIDs {
		ids = append(ids, u.String())
	}
	out := models.AlertRule{
		UUID:               r.Uuid.String(),
		Name:               r.Name,
		Description:        r.Description,
		Source:             r.Source,
		Params:             r.Params,
		Severity:           r.Severity,
		Enabled:            r.Enabled,
		EvaluationInterval: int(r.EvaluationIntervalSeconds),
		For:                int(r.ForSeconds),
		RepeatInterval:     int(r.RepeatIntervalSeconds),
		TargetIDs:          ids,
		CreatedAt:          r.CreatedAt,
		UpdatedAt:          r.UpdatedAt,
	}
	if r.LastEvaluatedAt.Valid {
		t := r.LastEvaluatedAt.Time
		out.LastEvaluatedAt = &t
	}
	return out, nil
}

func normalizeParams(p json.RawMessage) json.RawMessage {
	if len(p) == 0 {
		return json.RawMessage("{}")
	}
	return p
}

func pageBounds(req models.PageRequest) (count, page int) {
	count = req.Count
	if count <= 0 {
		count = 10
	}
	page = req.Page
	if page < 0 {
		page = 0
	}
	return count, page
}
