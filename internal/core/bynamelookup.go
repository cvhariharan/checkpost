package core

import (
	"context"

	"github.com/cvhariharan/checkpost/internal/models"
)

func (c *Core) PolicyUUIDByName(ctx context.Context, name string) (models.LookupResult, error) {
	row, err := c.store.GetPolicyByName(ctx, name)
	if err != nil {
		return models.LookupResult{}, err
	}
	return models.LookupResult{UUID: row.Uuid.String(), Name: row.Name}, nil
}

func (c *Core) ScheduleUUIDByName(ctx context.Context, name string) (models.LookupResult, error) {
	row, err := c.store.GetScheduleByName(ctx, name)
	if err != nil {
		return models.LookupResult{}, err
	}
	return models.LookupResult{UUID: row.Uuid.String(), Name: row.Name}, nil
}

func (c *Core) AlertRuleUUIDByName(ctx context.Context, name string) (models.LookupResult, error) {
	row, err := c.store.GetAlertRuleByName(ctx, name)
	if err != nil {
		return models.LookupResult{}, err
	}
	return models.LookupResult{UUID: row.Uuid.String(), Name: row.Name}, nil
}

func (c *Core) AlertTargetUUIDByName(ctx context.Context, name string) (models.LookupResult, error) {
	row, err := c.store.GetAlertTargetByName(ctx, name)
	if err != nil {
		return models.LookupResult{}, err
	}
	return models.LookupResult{UUID: row.Uuid.String(), Name: row.Name}, nil
}

func (c *Core) GroupUUIDByName(ctx context.Context, name string) (models.LookupResult, error) {
	row, err := c.store.GetGroupByName(ctx, name)
	if err != nil {
		return models.LookupResult{}, err
	}
	return models.LookupResult{UUID: row.Uuid.String(), Name: row.Name}, nil
}
