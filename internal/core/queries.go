package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/cvhariharan/watcher/internal/models"
	"github.com/cvhariharan/watcher/internal/repo"
	"github.com/google/uuid"
)

func (c *Core) CreateQuery(ctx context.Context, req models.CreateQuery) (models.Query, error) {
	if err := validateQuery(req.SQL); err != nil {
		return models.Query{}, err
	}

	q, err := c.store.CreateQuery(ctx, repo.CreateQueryParams{
		Name:        req.Name,
		Sql:         req.SQL,
		Description: req.Description,
		IsSystem:    req.IsSystem,
	})
	if err != nil {
		return models.Query{}, fmt.Errorf("create query: %w", err)
	}

	return toModelQuery(q), nil
}

func (c *Core) GetQuery(ctx context.Context, req models.ResourceID) (models.Query, error) {
	queryID, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.Query{}, fmt.Errorf("parse query uuid: %w", err)
	}

	q, err := c.store.GetQueryByUUID(ctx, queryID)
	if err != nil {
		return models.Query{}, fmt.Errorf("get query: %w", err)
	}

	return toModelQuery(q), nil
}

func (c *Core) PaginateQueries(ctx context.Context, req models.PageRequest) (models.Page[models.Query], error) {
	page := req.Page
	countPerPage := req.Count
	if countPerPage <= 0 {
		countPerPage = 10
	}
	if page < 0 {
		page = 0
	}

	rows, err := c.store.ListQueries(ctx, repo.ListQueriesParams{
		Limit:  int32(countPerPage),
		Offset: int32(page * countPerPage),
	})
	if err != nil {
		return models.Page[models.Query]{}, fmt.Errorf("list queries: %w", err)
	}

	out := make([]models.Query, 0, len(rows))
	totalCount := 0
	for _, row := range rows {
		out = append(out, toModelQueryRow(row))
		totalCount = int(row.TotalCount)
	}

	return models.Page[models.Query]{
		Items:      out,
		TotalCount: totalCount,
		PageCount:  pageCountFor(totalCount, countPerPage),
	}, nil
}

func (c *Core) UpdateQuery(ctx context.Context, req models.UpdateQuery) (models.Query, error) {
	if err := validateQuery(req.SQL); err != nil {
		return models.Query{}, err
	}

	queryID, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.Query{}, fmt.Errorf("parse query uuid: %w", err)
	}

	q, err := c.store.UpdateQueryTx(ctx, repo.UpdateQueryTxParams{
		Query: repo.UpdateQueryByUUIDParams{
			Uuid:        queryID,
			Name:        req.Name,
			Sql:         req.SQL,
			Description: req.Description,
		},
	})
	if err != nil {
		return models.Query{}, fmt.Errorf("update query: %w", err)
	}

	return toModelQuery(q), nil
}

func (c *Core) DeleteQuery(ctx context.Context, req models.ResourceID) error {
	queryID, err := uuid.Parse(req.UUID)
	if err != nil {
		return fmt.Errorf("parse query uuid: %w", err)
	}

	rows, err := c.store.DeleteQueryByUUID(ctx, queryID)
	if err != nil {
		return fmt.Errorf("delete query: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func validateQuery(query string) error {
	if strings.TrimSpace(query) == "" {
		return fmt.Errorf("%w: query cannot be empty", ErrInvalidQuery)
	}

	keywords := []string{"SELECT", "FROM", "WHERE", "JOIN", "ORDER BY", "GROUP BY", "HAVING", "LIMIT"}
	upper := strings.ToUpper(query)
	for _, keyword := range keywords {
		if strings.Contains(upper, keyword) {
			return nil
		}
	}

	return fmt.Errorf("%w: query does not appear to be valid SQL", ErrInvalidQuery)
}
