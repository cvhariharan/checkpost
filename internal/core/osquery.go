package core

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/cvhariharan/watcher/internal/models"
	"github.com/cvhariharan/watcher/internal/repo"
	"github.com/google/uuid"
)

type Core struct {
	store  repo.Store
	logger *slog.Logger
}

func NewCore(logger *slog.Logger, store repo.Store) *Core {
	return &Core{store: store, logger: logger.WithGroup("core.osquery")}
}

func ToNullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func (c *Core) EnrollNode(ctx context.Context, hostIdentifier string, details models.HostDetailsInfo) (string, error) {
	node := repo.Node{
		HostIdentifier: ToNullString(hostIdentifier),
		OsName:         ToNullString(details.OSVersion.Name),
	}

	osVersionInfo := repo.OsVersionInfo{
		OsID:         ToNullString(details.OSVersion.OSID),
		Codename:     ToNullString(details.OSVersion.Codename),
		Major:        ToNullString(details.OSVersion.Major),
		Minor:        ToNullString(details.OSVersion.Minor),
		Name:         ToNullString(details.OSVersion.Name),
		Patch:        ToNullString(details.OSVersion.Patch),
		Platform:     ToNullString(details.OSVersion.Platform),
		PlatformLike: ToNullString(details.OSVersion.PlatformLike),
		Version:      ToNullString(details.OSVersion.Version),
	}

	osqueryUUID, err := uuid.Parse(details.OSQuery.UUID)
	if err != nil {
		return "", fmt.Errorf("error parsing osquery UUID: %w", err)
	}
	osqueryInfo := repo.OsqueryInfo{
		BuildDistro:   ToNullString(details.OSQuery.BuildDistro),
		BuildPlatform: ToNullString(details.OSQuery.BuildPlatform),
		ConfigHash:    ToNullString(details.OSQuery.ConfigHash),
		ConfigValid:   ToNullString(details.OSQuery.ConfigValid),
		Extensions:    ToNullString(details.OSQuery.Extension),
		InstanceID:    ToNullString(details.OSQuery.InstanceID),
		Pid:           ToNullString(details.OSQuery.PID),
		StartTime:     ToNullString(details.OSQuery.StartTime),
		Uuid:          osqueryUUID,
		Version:       ToNullString(details.OSQuery.Version),
		Watcher:       ToNullString(details.OSQuery.Watcher),
	}

	platformInfo := repo.PlatformInfo{
		Address:    ToNullString(details.Platform.Address),
		Date:       ToNullString(details.Platform.Date),
		Extra:      ToNullString(details.Platform.Extra),
		Revision:   ToNullString(details.Platform.Revision),
		Size:       ToNullString(details.Platform.Size),
		Vendor:     ToNullString(details.Platform.Vendor),
		Version:    ToNullString(details.Platform.Version),
		VolumeSize: ToNullString(details.Platform.VolumeSize),
	}

	systemInfo := repo.SystemInfo{
		ComputerName:     ToNullString(details.System.ComputerName),
		CpuBrand:         ToNullString(details.System.CPUBrand),
		CpuLogicalCores:  ToNullString(details.System.CPULogicalCores),
		CpuPhysicalCores: ToNullString(details.System.CPUPhysicalCores),
		CpuSubtype:       ToNullString(details.System.CPUSubtype),
		CpuType:          ToNullString(details.System.CPUType),
		HardwareModel:    ToNullString(details.System.HardwareModel),
		HardwareSerial:   ToNullString(details.System.HardwareSerial),
		HardwareVendor:   ToNullString(details.System.HardwareVendor),
		HardwareVersion:  ToNullString(details.System.HardwareVersion),
		Hostname:         ToNullString(details.System.Hostname),
		LocalHostname:    ToNullString(details.System.LocalHostname),
		PhysicalMemory:   ToNullString(details.System.PhysicalMemory),
		Uuid:             uuid.New(),
	}

	return c.store.CreateNodeTx(ctx, node, osVersionInfo, osqueryInfo, platformInfo, systemInfo)
}
