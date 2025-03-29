package core

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/cvhariharan/watcher/internal/models"
	"github.com/cvhariharan/watcher/internal/repo"
)

type Core struct {
	store  *repo.Store
	logger *slog.Logger
}

func NewCore(logger *slog.Logger, store *repo.Store) *Core {
	return &Core{store: store, logger: logger.WithGroup("core")}
}

func ToNullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func (c *Core) EnrollNode(ctx context.Context, node models.Node) (string, error) {
	return c.store.CreateNode(ctx, node)
}

func (c *Core) CreateQuery(ctx context.Context, query, description string) (models.Query, error) {
	return c.store.CreateQuery(ctx, query, description)
}

func (c *Core) GetQuery(ctx context.Context, id string) (models.Query, error) {
	return c.store.GetQuery(ctx, id)
}

func (c *Core) PaginateQueries(ctx context.Context, page, countPerPage int) (queries []models.Query, totalCount int, pageCount int, err error) {
	return c.store.GetQueries(ctx, countPerPage, page*countPerPage)
}

func (c *Core) UpdateQuery(ctx context.Context, id, query, description string) (models.Query, error) {
	return c.store.UpdateQuery(ctx, id, query, description)
}

func (c *Core) DeleteQuery(ctx context.Context, id string) error {
	return c.store.DeleteQuery(ctx, id)
}

func (c *Core) CreateSchedule(ctx context.Context, sched models.Schedule, queryID string) (string, error) {
	return c.store.CreateSchedule(ctx, sched, queryID)
}
