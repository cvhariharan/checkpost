package repo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/cvhariharan/watcher/internal/models"
	"github.com/jmoiron/sqlx"
)

var (
	ErrNotFound = errors.New("not found")
)

// PreparedQueries holds all prepared SQL statements
type PreparedQueries struct {
	AddNode       *sqlx.Stmt `query:"add-node"`
	GetNodeByUUID *sqlx.Stmt `query:"get-node-by-uuid"`

	AddOSVersionInfo       *sqlx.Stmt `query:"add-os-version-info"`
	GetOSVersionInfoByNode *sqlx.Stmt `query:"get-os-version-info"`

	AddOSQueryInfo       *sqlx.Stmt `query:"add-osquery-info"`
	GetOSQueryInfoByNode *sqlx.Stmt `query:"get-osquery-info"`

	AddSystemInfo       *sqlx.Stmt `query:"add-system-info"`
	GetSystemInfoByNode *sqlx.Stmt `query:"get-system-info"`

	AddPlatformInfo       *sqlx.Stmt `query:"add-platform-info"`
	GetPlatformInfoByNode *sqlx.Stmt `query:"get-platform-info"`

	CreateQuery       *sqlx.Stmt `query:"create-query"`
	GetQueryByUUID    *sqlx.Stmt `query:"get-query-by-uuid"`
	DeleteQueryByUUID *sqlx.Stmt `query:"delete-query-by-uuid"`
	UpdateQueryByUUID *sqlx.Stmt `query:"update-query-by-uuid"`
	GetQueries        *sqlx.Stmt `query:"get-queries"`
	GetQuery          *sqlx.Stmt `query:"get-query"`

	CreateSchedule *sqlx.Stmt `query:"create-schedule"`
	GetSchedules   *sqlx.Stmt `query:"get-schedules"`
}

// Store represents the database store
type Store struct {
	db      *sqlx.DB
	queries PreparedQueries
	logger  *slog.Logger
}

// NewStore creates a new store instance
func NewPostgresStore(logger *slog.Logger, db *sqlx.DB, q PreparedQueries) *Store {
	return &Store{
		logger:  logger.WithGroup("repo"),
		db:      db,
		queries: q,
	}
}

// CreateNodeTx creates a node and all its related info in a transaction
func (s *Store) CreateNode(ctx context.Context, node models.Node) (string, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("error starting transaction: %w", err)
	}
	defer tx.Rollback()

	var nodeID int
	var nodeKey string

	addNodeStmt := tx.Stmtx(s.queries.AddNode)
	addOSVersionStmt := tx.Stmtx(s.queries.AddOSVersionInfo)
	addOSQueryStmt := tx.Stmtx(s.queries.AddOSQueryInfo)
	addSystemStmt := tx.Stmtx(s.queries.AddSystemInfo)
	addPlatformStmt := tx.Stmtx(s.queries.AddPlatformInfo)

	if err := addNodeStmt.QueryRowxContext(ctx, node.HostIdentifier, node.OSVersion.Name).Scan(&nodeID, &nodeKey); err != nil {
		return "", fmt.Errorf("error creating node: %w", err)
	}

	if _, err := addOSVersionStmt.ExecContext(
		ctx,
		node.OSVersion.OSID,
		node.OSVersion.Codename,
		node.OSVersion.Major,
		node.OSVersion.Minor,
		node.OSVersion.Name,
		node.OSVersion.Patch,
		node.OSVersion.Platform,
		node.OSVersion.PlatformLike,
		node.OSVersion.Version,
		nodeID,
	); err != nil {
		return "", fmt.Errorf("error adding os version info: %w", err)
	}

	if _, err := addOSQueryStmt.ExecContext(
		ctx,
		node.OSQuery.BuildDistro,
		node.OSQuery.BuildPlatform,
		node.OSQuery.ConfigHash,
		node.OSQuery.ConfigValid,
		node.OSQuery.Extension,
		node.OSQuery.InstanceID,
		node.OSQuery.PID,
		node.OSQuery.StartTime,
		node.OSQuery.OsqueryUUID,
		node.OSQuery.Version,
		node.OSQuery.Watcher,
		nodeID,
	); err != nil {
		return "", fmt.Errorf("error adding osquery info: %w", err)
	}

	if _, err := addSystemStmt.ExecContext(
		ctx,
		node.System.ComputerName,
		node.System.CPUBrand,
		node.System.CPULogicalCores,
		node.System.CPUPhysicalCores,
		node.System.CPUSubtype,
		node.System.CPUType,
		node.System.HardwareModel,
		node.System.HardwareSerial,
		node.System.HardwareVendor,
		node.System.HardwareVersion,
		node.System.Hostname,
		node.System.LocalHostname,
		node.System.PhysicalMemory,
		node.System.UUID,
		nodeID,
	); err != nil {
		return "", fmt.Errorf("error adding system info: %w", err)
	}

	if _, err := addPlatformStmt.ExecContext(
		ctx,
		node.Platform.Address,
		node.Platform.Date,
		node.Platform.Extra,
		node.Platform.Revision,
		node.Platform.Size,
		node.Platform.Vendor,
		node.Platform.Version,
		node.Platform.VolumeSize,
		nodeID,
	); err != nil {
		return "", fmt.Errorf("error adding platform info: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("error committing transaction: %w", err)
	}

	return nodeKey, nil
}

func (s *Store) CreateQuery(ctx context.Context, title, query, description string) (models.Query, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return models.Query{}, fmt.Errorf("error starting transaction: %w", err)
	}
	defer tx.Rollback()

	var q models.Query
	if err := tx.Stmtx(s.queries.CreateQuery).Get(&q, title, query, description); err != nil {
		return models.Query{}, fmt.Errorf("error creating query: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return models.Query{}, fmt.Errorf("error committing transaction: %w", err)
	}

	return q, nil
}

func (s *Store) GetQuery(ctx context.Context, queryUUID string) (models.Query, error) {
	var q models.Query
	if err := s.queries.GetQueryByUUID.Get(&q, queryUUID); err != nil {
		return models.Query{}, fmt.Errorf("error getting query %s: %w", queryUUID, err)
	}
	return q, nil
}

func (s *Store) GetQueries(ctx context.Context, limit, offset int) (queries []models.Query, count int, pageCount int, err error) {
	var q []models.Query
	rows, err := s.queries.GetQueries.QueryxContext(ctx, limit, offset)
	if err != nil {
		return nil, -1, -1, fmt.Errorf("error getting queries: %w", err)
	}
	defer rows.Close()

	totalCountVal := 0
	pageCountVal := 0

	for rows.Next() {
		var query models.Query
		var tc int
		var pc int

		if err := rows.Scan(&query.UUID, &query.Title, &query.Query, &query.Description, &pc, &tc); err != nil {
			return nil, -1, -1, fmt.Errorf("error scanning queries: %w", err)
		}

		q = append(q, query)

		totalCountVal = tc
		pageCountVal = pc
	}

	if err := rows.Err(); err != nil {
		return nil, -1, -1, fmt.Errorf("error iterating over queries: %w", err)
	}

	return q, totalCountVal, pageCountVal, nil
}

func (s *Store) DeleteQuery(ctx context.Context, queryUUID string) error {
	if _, err := s.queries.DeleteQueryByUUID.ExecContext(ctx, queryUUID); err != nil {
		return fmt.Errorf("error deleting query %s: %w", queryUUID, err)
	}
	return nil
}

func (s *Store) UpdateQuery(ctx context.Context, queryUUID, title, query, description string) (models.Query, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return models.Query{}, fmt.Errorf("error starting transaction: %w", err)
	}
	defer tx.Rollback()

	var q models.Query
	if err := tx.Stmtx(s.queries.UpdateQueryByUUID).Get(&q, title, query, description, queryUUID); err != nil {
		return models.Query{}, fmt.Errorf("error updating query: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return models.Query{}, fmt.Errorf("error committing transaction: %w", err)
	}

	return q, nil
}

func (s *Store) CreateSchedule(ctx context.Context, schedule models.Schedule, queryID string) (string, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("error starting transaction: %w", err)
	}
	defer tx.Rollback()

	var q models.Query
	if err := s.queries.GetQueryByUUID.Get(&q, queryID); err != nil {
		return "", fmt.Errorf("error getting query %s while creating schedule: %w", schedule.Query.UUID, err)
	}

	var sched models.Schedule
	if err := tx.Stmtx(s.queries.CreateSchedule).Get(
		&sched,
		q.ID,
		schedule.Interval,
		schedule.Platform,
		schedule.Version,
		schedule.Shard,
		schedule.Denylist,
		schedule.Removed,
		schedule.Snapshot,
		schedule.Title,
	); err != nil {
		return "", fmt.Errorf("error creating schedule: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("error committing transaction: %w", err)
	}

	return sched.UUID, nil
}

func (s *Store) GetSchedules(ctx context.Context, limit, offset int) (schedules []models.Schedule, count int, pageCount int, err error) {
	var q []models.Schedule
	rows, err := s.queries.GetSchedules.QueryxContext(ctx, limit, offset)
	if err != nil {
		return nil, -1, -1, fmt.Errorf("error getting schedules: %w", err)
	}
	defer rows.Close()

	totalCountVal := 0
	pageCountVal := 0

	for rows.Next() {
		var schedule models.Schedule
		var tc int
		var pc int

		if err := rows.Scan(
			&schedule.Title,
			&schedule.UUID,
			&schedule.Query_ID,
			&schedule.Interval,
			&schedule.Platform,
			&schedule.Version,
			&schedule.Shard,
			&schedule.Denylist,
			&schedule.Removed,
			&schedule.Snapshot,
			&pc,
			&tc,
		); err != nil {
			return nil, -1, -1, fmt.Errorf("error scanning schedule: %w", err)
		}

		var query models.Query
		if err := s.queries.GetQuery.Get(&query, schedule.Query_ID); err != nil {
			return nil, -1, -1, fmt.Errorf("error getting query for schedule %s: %w", schedule.UUID, err)
		}
		schedule.Query = query

		q = append(q, schedule)

		totalCountVal = tc
		pageCountVal = pc
	}

	if err := rows.Err(); err != nil {
		return nil, -1, -1, fmt.Errorf("error iterating over schedules: %w", err)
	}

	return q, totalCountVal, pageCountVal, nil
}
