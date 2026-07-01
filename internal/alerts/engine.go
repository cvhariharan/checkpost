package alerts

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/cvhariharan/checkpost/internal/repo"
)

const tickInterval = 30 * time.Second

// Store is the subset of repo access the engine needs.
type Store interface {
	ClaimDueAlertRules(ctx context.Context) ([]repo.AlertRule, error)
	ListTargetsForRule(ctx context.Context, ruleID int64) ([]repo.AlertTarget, error)
	ListAlertStateByRule(ctx context.Context, ruleID int64) ([]repo.AlertState, error)
	UpsertAlertState(ctx context.Context, arg repo.UpsertAlertStateParams) error
	DeleteAlertState(ctx context.Context, arg repo.DeleteAlertStateParams) error
}

type Engine struct {
	store  Store
	logger *slog.Logger

	mu     sync.Mutex
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

func NewEngine(store Store, logger *slog.Logger) *Engine {
	return &Engine{store: store, logger: logger.WithGroup("alerts.engine")}
}

func (e *Engine) Start() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	e.wg.Add(1)
	go e.run(ctx)
}

func (e *Engine) Close() error {
	e.mu.Lock()
	cancel := e.cancel
	e.cancel = nil
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	e.wg.Wait()
	return nil
}

func (e *Engine) run(ctx context.Context) {
	defer e.wg.Done()
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := e.tick(ctx); err != nil {
				e.logger.Error("alert tick failed", "error", err)
			}
		}
	}
}

func (e *Engine) tick(ctx context.Context) error {
	// ClaimDueAlertRules atomically bumps last_evaluated_at as it selects, so a
	// rule is claimed by at most one instance per interval. The trade-off is that
	// a rule is marked evaluated before evaluateRule runs: a failed evaluation is
	// not retried until the next interval rather than on the next tick.
	rules, err := e.store.ClaimDueAlertRules(ctx)
	if err != nil {
		return err
	}
	for _, rule := range rules {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := e.evaluateRule(ctx, rule); err != nil {
			e.logger.Error("evaluate rule", "rule", rule.Name, "error", err)
		}
	}
	return nil
}

func (e *Engine) evaluateRule(ctx context.Context, rule repo.AlertRule) error {
	source, ok := LookupSource(rule.Source)
	if !ok {
		e.logger.Warn("unknown alert source", "rule", rule.Name, "source", rule.Source)
		return nil
	}

	matches, err := source.Evaluate(ctx, rule.Params)
	if err != nil {
		return err
	}
	current := make(map[string]Alert, len(matches))
	for _, a := range matches {
		current[a.Key] = a
	}

	stateRows, err := e.store.ListAlertStateByRule(ctx, rule.ID)
	if err != nil {
		return err
	}
	states := make(map[string]repo.AlertState, len(stateRows))
	for _, s := range stateRows {
		states[s.AlertKey] = s
	}

	now := time.Now().UTC()
	forDur := time.Duration(rule.ForSeconds) * time.Second
	repeatDur := time.Duration(rule.RepeatIntervalSeconds) * time.Second

	var firing, resolved []Alert

	for key, a := range current {
		st, exists := states[key]
		if !exists {
			status := "pending"
			notified := sql.NullTime{}
			if forDur == 0 {
				status = "firing"
				firing = append(firing, a)
				notified = sql.NullTime{Time: now, Valid: true}
			}
			if err := e.upsert(ctx, rule.ID, key, status, a, now, now, notified); err != nil {
				return err
			}
			continue
		}

		if st.Status == "pending" {
			if now.Sub(st.FirstSeenAt) >= forDur {
				firing = append(firing, a)
				if err := e.upsert(ctx, rule.ID, key, "firing", a, st.FirstSeenAt, now, sql.NullTime{Time: now, Valid: true}); err != nil {
					return err
				}
			} else {
				if err := e.upsert(ctx, rule.ID, key, "pending", a, st.FirstSeenAt, now, st.LastNotifiedAt); err != nil {
					return err
				}
			}
			continue
		}

		// firing key still present
		notified := st.LastNotifiedAt
		if repeatDur > 0 && (!st.LastNotifiedAt.Valid || now.Sub(st.LastNotifiedAt.Time) >= repeatDur) {
			firing = append(firing, a)
			notified = sql.NullTime{Time: now, Valid: true}
		}
		if err := e.upsert(ctx, rule.ID, key, "firing", a, st.FirstSeenAt, now, notified); err != nil {
			return err
		}
	}

	for key, st := range states {
		if _, ok := current[key]; ok {
			continue
		}
		if st.Status == "firing" {
			resolved = append(resolved, alertFromState(st))
		}
		if err := e.store.DeleteAlertState(ctx, repo.DeleteAlertStateParams{RuleID: rule.ID, AlertKey: key}); err != nil {
			return err
		}
	}

	if len(firing) > 0 || len(resolved) > 0 {
		targets, err := e.store.ListTargetsForRule(ctx, rule.ID)
		if err != nil {
			return err
		}
		e.dispatch(ctx, rule, targets, EventFiring, firing)
		e.dispatch(ctx, rule, targets, EventResolved, resolved)
	}

	return nil
}

// dispatch hands the full alert set to each target's notifier. Each notifier
// owns how it groups deliveries: webhooks fire one request per host while the
// SMTP target bundles alerts sharing a recipient set into a single email.
func (e *Engine) dispatch(ctx context.Context, rule repo.AlertRule, targets []repo.AlertTarget, kind EventKind, alerts []Alert) {
	if len(alerts) == 0 || len(targets) == 0 {
		return
	}

	r := Rule{
		UUID: rule.Uuid.String(), Name: rule.Name, Description: rule.Description,
		Source: rule.Source, Severity: rule.Severity, Params: rule.Params,
	}
	for _, t := range targets {
		notifier, ok := LookupNotifier(t.Type)
		if !ok {
			e.logger.Warn("unknown notifier", "type", t.Type)
			continue
		}
		target := Target{UUID: t.Uuid.String(), Name: t.Name, Type: t.Type, Config: t.Config}
		if err := notifier.Send(ctx, kind, target, alerts, r); err != nil {
			e.logger.Error("send alert", "rule", rule.Name, "target", t.Name, "kind", kind, "error", err)
		}
	}
}

func (e *Engine) upsert(ctx context.Context, ruleID int64, key, status string, a Alert, firstSeen, lastSeen time.Time, notified sql.NullTime) error {
	labels, _ := json.Marshal(a.Labels)
	annotations, _ := json.Marshal(a.Annotations)
	return e.store.UpsertAlertState(ctx, repo.UpsertAlertStateParams{
		RuleID:         ruleID,
		AlertKey:       key,
		Status:         status,
		Labels:         labels,
		Annotations:    annotations,
		FirstSeenAt:    firstSeen,
		LastSeenAt:     lastSeen,
		LastNotifiedAt: notified,
	})
}

func alertFromState(st repo.AlertState) Alert {
	a := Alert{Key: st.AlertKey}
	_ = json.Unmarshal(st.Labels, &a.Labels)
	_ = json.Unmarshal(st.Annotations, &a.Annotations)
	return a
}
