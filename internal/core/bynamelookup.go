package core

import "context"

// LookupResult is the minimal by-name lookup payload used by `watcher apply` to
// decide create vs update. It deliberately carries only the natural key and the
// UUID needed to address the resource on update.
type LookupResult struct {
	UUID string
	Name string
}

// The by-name lookups below return the underlying store error verbatim so the
// handler can map sql.ErrNoRows to a 404 (absent → the reconciler creates it).

func (c *Core) PolicyUUIDByName(ctx context.Context, name string) (LookupResult, error) {
	row, err := c.store.GetPolicyByName(ctx, name)
	if err != nil {
		return LookupResult{}, err
	}
	return LookupResult{UUID: row.Uuid.String(), Name: row.Name}, nil
}

func (c *Core) ScheduleUUIDByName(ctx context.Context, name string) (LookupResult, error) {
	row, err := c.store.GetScheduleByName(ctx, name)
	if err != nil {
		return LookupResult{}, err
	}
	return LookupResult{UUID: row.Uuid.String(), Name: row.Name}, nil
}

func (c *Core) AlertRuleUUIDByName(ctx context.Context, name string) (LookupResult, error) {
	row, err := c.store.GetAlertRuleByName(ctx, name)
	if err != nil {
		return LookupResult{}, err
	}
	return LookupResult{UUID: row.Uuid.String(), Name: row.Name}, nil
}

func (c *Core) AlertTargetUUIDByName(ctx context.Context, name string) (LookupResult, error) {
	row, err := c.store.GetAlertTargetByName(ctx, name)
	if err != nil {
		return LookupResult{}, err
	}
	return LookupResult{UUID: row.Uuid.String(), Name: row.Name}, nil
}

func (c *Core) GroupUUIDByName(ctx context.Context, name string) (LookupResult, error) {
	row, err := c.store.GetGroupByName(ctx, name)
	if err != nil {
		return LookupResult{}, err
	}
	return LookupResult{UUID: row.Uuid.String(), Name: row.Name}, nil
}
