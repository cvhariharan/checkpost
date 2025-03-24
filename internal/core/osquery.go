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
	return &Core{store: store, logger: logger.WithGroup("core.osquery")}
}

func ToNullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func (c *Core) EnrollNode(ctx context.Context, node models.Node) (string, error) {
	return c.store.CreateNodeTx(ctx, node)
}
