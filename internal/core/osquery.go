package core

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/cvhariharan/watcher/internal/logqueue"
	"github.com/cvhariharan/watcher/internal/models"
	"github.com/cvhariharan/watcher/internal/repo"
)

const MaxLogCount = 1000

type Core struct {
	store     *repo.Store
	logger    *slog.Logger
	logqueuer *logqueue.StreamLogger

	// systemSchedules is a map of scheduled queries created by watcher itself
	systemSchedulesMap map[string]bool
}

func NewCore(logger *slog.Logger, store *repo.Store, logqueuer *logqueue.StreamLogger) (*Core, error) {
	s, err := store.GetSystemSchedules(context.Background())
	if err != nil {
		return nil, err
	}

	m := make(map[string]bool)
	for _, sched := range s {
		m[sched] = true
	}
	return &Core{store: store, logger: logger.WithGroup("core"), logqueuer: logqueuer, systemSchedulesMap: m}, nil
}

func ToNullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func (c *Core) EnrollNode(ctx context.Context, node models.Node) (string, error) {
	return c.store.CreateNode(ctx, node)
}

func (c *Core) GetNode(ctx context.Context, id string) (models.Node, error) {
	return c.store.GetNode(ctx, id)
}

func (c *Core) CreateQuery(ctx context.Context, title, query, description string) (models.Query, error) {
	return c.store.CreateQuery(ctx, title, query, description)
}

func (c *Core) GetQuery(ctx context.Context, id string) (models.Query, error) {
	return c.store.GetQuery(ctx, id)
}

func (c *Core) PaginateQueries(ctx context.Context, page, countPerPage int) (queries []models.Query, totalCount int, pageCount int, err error) {
	return c.store.GetQueries(ctx, countPerPage, page*countPerPage)
}

func (c *Core) UpdateQuery(ctx context.Context, id, title, query, description string) (models.Query, error) {
	return c.store.UpdateQuery(ctx, id, title, query, description)
}

func (c *Core) DeleteQuery(ctx context.Context, id string) error {
	return c.store.DeleteQuery(ctx, id)
}

func (c *Core) CreateSchedule(ctx context.Context, sched models.Schedule, queryID string) (string, error) {
	return c.store.CreateSchedule(ctx, sched, queryID)
}

func (c *Core) PaginateSchedules(ctx context.Context, page, countPerPage int) (schedules []models.Schedule, totalCount int, pageCount int, err error) {
	return c.store.GetSchedules(ctx, countPerPage, page*countPerPage)
}

func (c *Core) GetSchedule(ctx context.Context, id string) (models.Schedule, error) {
	return c.store.GetSchedule(ctx, id)
}

func (c *Core) DeleteSchedule(ctx context.Context, scheduleID string) error {
	return c.store.DeleteSchedule(ctx, scheduleID)
}

func (c *Core) UpdateSchedule(ctx context.Context, sched models.Schedule, queryID string) (models.Schedule, error) {
	return c.store.UpdateSchedule(ctx, sched, queryID)
}

func (c *Core) LogResults(ctx context.Context, msgType string, data []map[string]interface{}) error {
	logsToProcess := data
	if len(data) > MaxLogCount {
		logsToProcess = data[:MaxLogCount]
		c.logger.Warn("log data truncated due to limit", "original_count", len(data), "max_count", MaxLogCount)
	}

	for _, l := range logsToProcess {
		var systemLog bool
		if nameVal, ok := l["name"]; ok {
			if nameStr, ok := nameVal.(string); ok {
				if c.systemSchedulesMap[nameStr] {
					systemLog = true
				}
			}
		}
		if err := c.logqueuer.WriteLog(ctx, logqueue.LogMsg{LogType: msgType, Data: data, SystemLog: systemLog}); err != nil {
			return err
		}
	}
	return nil
}
