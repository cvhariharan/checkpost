package core

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cvhariharan/watcher/internal/models"
	"github.com/cvhariharan/watcher/internal/repo"
	"github.com/google/uuid"
)

func (c *Core) CreateSchedule(ctx context.Context, req models.CreateSchedule) (models.Schedule, error) {
	queryID, err := uuid.Parse(req.QueryUUID)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("parse query uuid: %w", err)
	}

	q, err := c.store.GetQueryByUUID(ctx, queryID)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("get query for schedule: %w", err)
	}

	params := repo.CreateScheduleParams{
		QueryID:         q.ID,
		Name:            req.Name,
		IntervalSeconds: int32(req.IntervalSeconds),
		Platform:        defaultString(req.Platform, "all"),
		Version:         req.Version,
		Shard:           int32(defaultInt(req.Shard, 100)),
		Denylist:        req.Denylist,
		Removed:         req.Removed,
		Snapshot:        req.Snapshot,
		Enabled:         defaultBool(req.Enabled, true),
		IsSystem:        req.IsSystem,
	}

	sched, err := c.store.CreateSchedule(ctx, params)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("create schedule: %w", err)
	}

	return toModelSchedule(sched, toModelQuery(q)), nil
}

func (c *Core) PaginateSchedules(ctx context.Context, req models.PageRequest) (models.Page[models.Schedule], error) {
	page := req.Page
	countPerPage := req.Count
	if countPerPage <= 0 {
		countPerPage = 10
	}
	if page < 0 {
		page = 0
	}

	rows, err := c.store.ListSchedulesWithQueries(ctx, repo.ListSchedulesWithQueriesParams{
		Limit:  int32(countPerPage),
		Offset: int32(page * countPerPage),
	})
	if err != nil {
		return models.Page[models.Schedule]{}, fmt.Errorf("list schedules: %w", err)
	}

	out := make([]models.Schedule, 0, len(rows))
	totalCount := 0
	for _, row := range rows {
		out = append(out, toModelScheduleRow(row))
		totalCount = int(row.TotalCount)
	}

	return models.Page[models.Schedule]{
		Items:      out,
		TotalCount: totalCount,
		PageCount:  pageCountFor(totalCount, countPerPage),
	}, nil
}

func (c *Core) ListEnabledSchedules(ctx context.Context, req models.ScheduleListRequest) ([]models.Schedule, error) {
	rows, err := c.store.ListEnabledSchedulesWithQueries(ctx, int32(req.Limit))
	if err != nil {
		return nil, fmt.Errorf("list enabled schedules: %w", err)
	}

	out := make([]models.Schedule, 0, len(rows))
	for _, row := range rows {
		out = append(out, toModelEnabledScheduleRow(row))
	}
	return out, nil
}

func (c *Core) GetSchedule(ctx context.Context, req models.ResourceID) (models.Schedule, error) {
	scheduleID, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("parse schedule uuid: %w", err)
	}

	sched, err := c.store.GetScheduleWithQueryByUUID(ctx, scheduleID)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("get schedule: %w", err)
	}

	return toModelScheduleWithQueryRow(sched), nil
}

func (c *Core) DeleteSchedule(ctx context.Context, req models.ResourceID) error {
	id, err := uuid.Parse(req.UUID)
	if err != nil {
		return fmt.Errorf("parse schedule uuid: %w", err)
	}

	rows, err := c.store.DeleteScheduleByUUID(ctx, id)
	if err != nil {
		return fmt.Errorf("delete schedule: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (c *Core) UpdateSchedule(ctx context.Context, req models.UpdateSchedule) (models.Schedule, error) {
	scheduleID, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("parse schedule uuid: %w", err)
	}

	queryID, err := uuid.Parse(req.QueryUUID)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("parse query uuid: %w", err)
	}

	q, err := c.store.GetQueryByUUID(ctx, queryID)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("get query for schedule: %w", err)
	}

	sched, err := c.store.UpdateScheduleByUUID(ctx, repo.UpdateScheduleByUUIDParams{
		Uuid:            scheduleID,
		QueryID:         q.ID,
		Name:            req.Name,
		IntervalSeconds: int32(req.IntervalSeconds),
		Platform:        defaultString(req.Platform, "all"),
		Version:         req.Version,
		Shard:           int32(defaultInt(req.Shard, 100)),
		Denylist:        req.Denylist,
		Removed:         req.Removed,
		Snapshot:        req.Snapshot,
		Enabled:         defaultBool(req.Enabled, true),
	})
	if err != nil {
		return models.Schedule{}, fmt.Errorf("update schedule: %w", err)
	}

	return toModelSchedule(sched, toModelQuery(q)), nil
}
