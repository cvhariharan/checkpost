package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/cvhariharan/checkpost/internal/repo"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

const (
	VisibilityPrivate = "private"
	VisibilityPublic  = "public"
)

var (
	// ErrSavedQueryNameTaken is returned when a saved query name collides with an
	// existing one owned by the same user (case-insensitive).
	ErrSavedQueryNameTaken = errors.New("you already have a saved query with this name")
	// ErrSavedQueryForbidden is returned when a user tries to modify a saved query
	// they do not own.
	ErrSavedQueryForbidden = errors.New("saved query belongs to another user")
	// ErrSavedQueryOwnerRequired is returned when the viewer cannot be resolved to a
	// user, so the query would be stored with a NULL owner and orphaned.
	ErrSavedQueryOwnerRequired = errors.New("saved query owner could not be resolved")
)

func (c *Core) CreateSavedQuery(ctx context.Context, viewerUUID string, req models.SavedQueryRequest) (models.SavedQuery, error) {
	fields, err := c.validateSavedQuery(req)
	if err != nil {
		return models.SavedQuery{}, err
	}

	viewerID := c.resolveCreatedBy(ctx, viewerUUID)
	if !viewerID.Valid {
		return models.SavedQuery{}, ErrSavedQueryOwnerRequired
	}
	row, err := c.store.CreateSavedQuery(ctx, repo.CreateSavedQueryParams{
		Name:        fields.name,
		Description: fields.description,
		Query:       fields.query,
		Targets:     fields.targets,
		Visibility:  fields.visibility,
		CreatedBy:   viewerID,
	})
	if err != nil {
		return models.SavedQuery{}, mapSavedQueryError(err)
	}
	return c.savedQueryModel(ctx, row.Uuid, viewerID)
}

func (c *Core) UpdateSavedQuery(ctx context.Context, viewerUUID, id string, req models.SavedQueryRequest) (models.SavedQuery, error) {
	queryUUID, err := uuid.Parse(id)
	if err != nil {
		return models.SavedQuery{}, fmt.Errorf("parse saved query uuid: %w", err)
	}
	fields, err := c.validateSavedQuery(req)
	if err != nil {
		return models.SavedQuery{}, err
	}

	viewerID := c.resolveCreatedBy(ctx, viewerUUID)
	existing, err := c.store.GetSavedQueryByUUID(ctx, queryUUID)
	if err != nil {
		return models.SavedQuery{}, fmt.Errorf("get saved query: %w", err)
	}
	if !ownsSavedQuery(existing.CreatedBy, viewerID) {
		return models.SavedQuery{}, ErrSavedQueryForbidden
	}

	if _, err := c.store.UpdateSavedQueryByUUID(ctx, repo.UpdateSavedQueryByUUIDParams{
		Uuid:        queryUUID,
		Name:        fields.name,
		Description: fields.description,
		Query:       fields.query,
		Targets:     fields.targets,
		Visibility:  fields.visibility,
	}); err != nil {
		return models.SavedQuery{}, mapSavedQueryError(err)
	}
	return c.savedQueryModel(ctx, queryUUID, viewerID)
}

func (c *Core) GetSavedQuery(ctx context.Context, viewerUUID string, req models.ResourceID) (models.SavedQuery, error) {
	queryUUID, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.SavedQuery{}, fmt.Errorf("parse saved query uuid: %w", err)
	}
	viewerID := c.resolveCreatedBy(ctx, viewerUUID)
	row, err := c.store.GetSavedQueryByUUID(ctx, queryUUID)
	if err != nil {
		return models.SavedQuery{}, fmt.Errorf("get saved query: %w", err)
	}
	if row.Visibility != VisibilityPublic && !ownsSavedQuery(row.CreatedBy, viewerID) {
		return models.SavedQuery{}, sql.ErrNoRows
	}
	return toModelSavedQuery(row, viewerID), nil
}

func (c *Core) savedQueryModel(ctx context.Context, queryUUID uuid.UUID, viewerID sql.NullInt64) (models.SavedQuery, error) {
	row, err := c.store.GetSavedQueryByUUID(ctx, queryUUID)
	if err != nil {
		return models.SavedQuery{}, fmt.Errorf("get saved query: %w", err)
	}
	return toModelSavedQuery(row, viewerID), nil
}

func (c *Core) ListSavedQueries(ctx context.Context, viewerUUID string, req models.PageRequest) (models.Page[models.SavedQuery], error) {
	countPerPage := req.Count
	if countPerPage <= 0 {
		countPerPage = 10
	}
	page := req.Page
	if page < 0 {
		page = 0
	}

	viewerID := c.resolveCreatedBy(ctx, viewerUUID)
	query := strings.TrimSpace(req.Query)
	totalCount, err := c.store.CountSavedQueries(ctx, repo.CountSavedQueriesParams{
		ViewerID: viewerID,
		Query:    query,
	})
	if err != nil {
		return models.Page[models.SavedQuery]{}, fmt.Errorf("count saved queries: %w", err)
	}
	rows, err := c.store.ListSavedQueries(ctx, repo.ListSavedQueriesParams{
		ViewerID:    viewerID,
		Query:       query,
		LimitCount:  int32(countPerPage),
		OffsetCount: int32(page * countPerPage),
	})
	if err != nil {
		return models.Page[models.SavedQuery]{}, fmt.Errorf("list saved queries: %w", err)
	}

	out := make([]models.SavedQuery, 0, len(rows))
	for _, row := range rows {
		out = append(out, toModelSavedQueryListRow(row, viewerID))
	}
	return models.Page[models.SavedQuery]{
		Items:      out,
		TotalCount: int(totalCount),
		PageCount:  pageCountFor(int(totalCount), countPerPage),
	}, nil
}

func (c *Core) DeleteSavedQuery(ctx context.Context, viewerUUID string, req models.ResourceID) error {
	queryUUID, err := uuid.Parse(req.UUID)
	if err != nil {
		return fmt.Errorf("parse saved query uuid: %w", err)
	}

	viewerID := c.resolveCreatedBy(ctx, viewerUUID)
	existing, err := c.store.GetSavedQueryByUUID(ctx, queryUUID)
	if err != nil {
		return fmt.Errorf("get saved query: %w", err)
	}
	if !ownsSavedQuery(existing.CreatedBy, viewerID) {
		return ErrSavedQueryForbidden
	}

	if _, err := c.store.DeleteSavedQueryByUUID(ctx, queryUUID); err != nil {
		return fmt.Errorf("delete saved query: %w", err)
	}
	return nil
}

type savedQueryFields struct {
	name        string
	description string
	query       string
	targets     string
	visibility  string
}

func (c *Core) validateSavedQuery(req models.SavedQueryRequest) (savedQueryFields, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return savedQueryFields{}, errors.New("name is required")
	}
	query := strings.TrimSpace(req.Query)
	if err := validateQuery(query); err != nil {
		return savedQueryFields{}, err
	}
	visibility := strings.TrimSpace(req.Visibility)
	if visibility == "" {
		visibility = VisibilityPrivate
	}
	if visibility != VisibilityPrivate && visibility != VisibilityPublic {
		return savedQueryFields{}, fmt.Errorf("invalid visibility %q", req.Visibility)
	}
	targets, err := json.Marshal(req.Targets)
	if err != nil {
		return savedQueryFields{}, fmt.Errorf("marshal query targets: %w", err)
	}
	return savedQueryFields{
		name:        name,
		description: strings.TrimSpace(req.Description),
		query:       query,
		targets:     string(targets),
		visibility:  visibility,
	}, nil
}

func mapSavedQueryError(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return ErrSavedQueryNameTaken
	}
	return fmt.Errorf("saved query: %w", err)
}
