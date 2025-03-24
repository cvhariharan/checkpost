package repo

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type Store interface {
	Querier
	CreateNodeTx(ctx context.Context, node Node, osVersionInfo OsVersionInfo, osqueryInfo OsqueryInfo, platformInfo PlatformInfo, systemInfo SystemInfo) (string, error)
}

type PostgresStore struct {
	*Queries
	db *sqlx.DB
}

func NewPostgresStore(db *sqlx.DB) Store {
	return &PostgresStore{
		db:      db,
		Queries: New(db),
	}
}

// CreateNodeTx creates a new node with the given information in a db transaction and returns the uuid (node key).
func (p *PostgresStore) CreateNodeTx(ctx context.Context, node Node, osVersionInfo OsVersionInfo, osqueryInfo OsqueryInfo, platformInfo PlatformInfo, systemInfo SystemInfo) (string, error) {
	tx, err := p.db.Begin()
	if err != nil {
		return "", fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback()

	q := Queries{db: tx}

	n, err := q.AddNode(ctx, AddNodeParams{
		HostIdentifier: node.HostIdentifier,
		OsName:         node.OsName,
	})
	if err != nil {
		return "", fmt.Errorf("error adding node: %w", err)
	}

	if _, err = q.AddOSVersionInfo(ctx, AddOSVersionInfoParams{
		OsID:         osVersionInfo.OsID,
		Codename:     osVersionInfo.Codename,
		Major:        osVersionInfo.Major,
		Minor:        osVersionInfo.Minor,
		Name:         osVersionInfo.Name,
		Patch:        osVersionInfo.Patch,
		Platform:     osVersionInfo.Platform,
		PlatformLike: osVersionInfo.PlatformLike,
		Version:      osVersionInfo.Version,
		NodeFk:       n.ID,
	}); err != nil {
		return "", fmt.Errorf("error adding os version info: %w", err)
	}

	if _, err = q.AddOSQueryInfo(ctx, AddOSQueryInfoParams{
		BuildDistro:   osqueryInfo.BuildDistro,
		BuildPlatform: osqueryInfo.BuildPlatform,
		ConfigHash:    osqueryInfo.ConfigHash,
		ConfigValid:   osqueryInfo.ConfigValid,
		Extensions:    osqueryInfo.Extensions,
		InstanceID:    osqueryInfo.InstanceID,
		Pid:           osqueryInfo.Pid,
		StartTime:     osqueryInfo.StartTime,
		Uuid:          osqueryInfo.Uuid,
		Version:       osqueryInfo.Version,
		Watcher:       osqueryInfo.Watcher,
		NodeFk:        n.ID,
	}); err != nil {
		return "", fmt.Errorf("error adding osquery info: %w", err)
	}

	if _, err = q.AddPlatformInfo(ctx, AddPlatformInfoParams{
		Address:    platformInfo.Address,
		Date:       platformInfo.Date,
		Extra:      platformInfo.Extra,
		Revision:   platformInfo.Revision,
		Size:       platformInfo.Size,
		Vendor:     platformInfo.Vendor,
		Version:    platformInfo.Version,
		VolumeSize: platformInfo.VolumeSize,
		NodeFk:     n.ID,
	}); err != nil {
		return "", fmt.Errorf("error adding platform info: %w", err)
	}

	if _, err = q.AddSystemInfo(ctx, AddSystemInfoParams{
		ComputerName:     systemInfo.ComputerName,
		CpuBrand:         systemInfo.CpuBrand,
		CpuLogicalCores:  systemInfo.CpuLogicalCores,
		CpuPhysicalCores: systemInfo.CpuPhysicalCores,
		CpuSubtype:       systemInfo.CpuSubtype,
		CpuType:          systemInfo.CpuType,
		HardwareModel:    systemInfo.HardwareModel,
		HardwareSerial:   systemInfo.HardwareSerial,
		HardwareVendor:   systemInfo.HardwareVendor,
		HardwareVersion:  systemInfo.HardwareVersion,
		Hostname:         systemInfo.Hostname,
		LocalHostname:    systemInfo.LocalHostname,
		PhysicalMemory:   systemInfo.PhysicalMemory,
		Uuid:             systemInfo.Uuid,
		NodeFk:           n.ID,
	}); err != nil {
		return "", fmt.Errorf("error adding system info: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("could not commit transaction: %w", err)
	}

	return n.Uuid.String(), nil
}
