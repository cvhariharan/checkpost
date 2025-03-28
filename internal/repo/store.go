package repo

import (
	"context"
	"encoding/json"
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

	CreateQuery    *sqlx.Stmt `query:"create-query"`
	GetQueryByUUID *sqlx.Stmt `query:"get-query-by-uuid"`
	GetQueries     *sqlx.Stmt `query:"get-queries"`
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
func (s *Store) CreateNodeTx(ctx context.Context, node models.Node) (string, error) {
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

// GetNodeByUUID retrieves a node and all its related info by UUID
func (s *Store) GetNodeByUUID(ctx context.Context, uuid string) (*models.Node, error) {
	row := s.queries.GetNodeByUUID.QueryRowContext(ctx, uuid)

	var node models.Node
	var osVersionJSON, osqueryJSON, systemJSON, platformJSON []byte

	err := row.Scan(
		&node.NodeKey,
		&node.HostIdentifier,
		&osVersionJSON,
		&osqueryJSON,
		&systemJSON,
		&platformJSON,
	)
	if err != nil {
		return nil, err
	}

	// Unmarshal JSON data into struct fields
	if err := json.Unmarshal(osVersionJSON, &node.OSVersion); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(osqueryJSON, &node.OSQuery); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(systemJSON, &node.System); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(platformJSON, &node.Platform); err != nil {
		return nil, err
	}

	return &node, nil
}

// AddOSVersionInfo adds OS version information for a node
func (s *Store) AddOSVersionInfo(ctx context.Context, info *models.OSVersionInfo, nodeFk int) error {
	_, err := s.queries.AddOSVersionInfo.ExecContext(ctx,
		info.OSID,
		info.Codename,
		info.Major,
		info.Minor,
		info.Name,
		info.Patch,
		info.Platform,
		info.PlatformLike,
		info.Version,
		nodeFk,
	)
	return err
}

// AddOSQueryInfo adds OSQuery information for a node
func (s *Store) AddOSQueryInfo(ctx context.Context, info *models.OsqueryInfo, nodeFk int) error {
	_, err := s.queries.AddOSQueryInfo.ExecContext(ctx,
		info.BuildDistro,
		info.BuildPlatform,
		info.ConfigHash,
		info.ConfigValid,
		info.Extension,
		info.InstanceID,
		info.PID,
		info.StartTime,
		info.OsqueryUUID,
		info.Version,
		info.Watcher,
		nodeFk,
	)
	return err
}

// AddSystemInfo adds system information for a node
func (s *Store) AddSystemInfo(ctx context.Context, info *models.SystemInfo, nodeFk int) error {
	_, err := s.queries.AddSystemInfo.ExecContext(ctx,
		info.ComputerName,
		info.CPUBrand,
		info.CPULogicalCores,
		info.CPUPhysicalCores,
		info.CPUSubtype,
		info.CPUType,
		info.HardwareModel,
		info.HardwareSerial,
		info.HardwareVendor,
		info.HardwareVersion,
		info.Hostname,
		info.LocalHostname,
		info.PhysicalMemory,
		info.UUID,
		nodeFk,
	)
	return err
}

// AddPlatformInfo adds platform information for a node
func (s *Store) AddPlatformInfo(ctx context.Context, info *models.PlatformInfo, nodeFk int) error {
	_, err := s.queries.AddPlatformInfo.ExecContext(ctx,
		info.Address,
		info.Date,
		info.Extra,
		info.Revision,
		info.Size,
		info.Vendor,
		info.Version,
		info.VolumeSize,
		nodeFk,
	)
	return err
}

func (s *Store) CreateQuery(ctx context.Context, query, description string) (models.Query, error) {
	var q models.Query
	if err := s.queries.CreateQuery.Get(&q, query, description); err != nil {
		return models.Query{}, err
	}

	return q, nil
}

func (s *Store) GetQueries(ctx context.Context, limit, offset int) (queries []models.Query, count int, pageCount int, err error) {
	var q []models.Query
	rows, err := s.queries.GetQueries.QueryxContext(ctx, limit, offset)
	if err != nil {
		return nil, -1, -1, err
	}
	defer rows.Close()

	totalCountVal := 0
	pageCountVal := 0

	for rows.Next() {
		var query models.Query
		var tc int
		var pc int

		if err := rows.Scan(&query.UUID, &query.Query, &query.Description, &pc, &tc); err != nil {
			return nil, -1, -1, err
		}

		q = append(q, query)

		totalCountVal = tc
		pageCountVal = pc
	}

	if err := rows.Err(); err != nil {
		return nil, -1, -1, err
	}

	return q, totalCountVal, pageCountVal, nil
}
