package core

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/cvhariharan/watcher/internal/models"
	"github.com/cvhariharan/watcher/internal/repo"
)

func toModelNode(node repo.Node) models.Node {
	var lastSeen *time.Time
	if node.LastSeenAt.Valid {
		lastSeen = &node.LastSeenAt.Time
	}
	var lastPolicyCheckAt *time.Time
	if node.LastPolicyCheckAt.Valid {
		lastPolicyCheckAt = &node.LastPolicyCheckAt.Time
	}

	return models.Node{
		ID:                node.ID,
		ResourceID:        node.Uuid.String(),
		UUID:              node.Uuid.String(),
		NodeKey:           node.NodeKey.String(),
		HostIdentifier:    node.HostIdentifier,
		Hostname:          node.Hostname,
		Platform:          node.Platform,
		OSName:            node.OsName,
		OSVersion:         node.OsVersion,
		OSQueryVersion:    node.OsqueryVersion,
		HardwareSerial:    node.HardwareSerial,
		EnrolledAt:        node.EnrolledAt,
		LastSeenAt:        lastSeen,
		LastPolicyCheckAt: lastPolicyCheckAt,
		CreatedAt:         node.CreatedAt,
		UpdatedAt:         node.UpdatedAt,
	}
}

func toModelNodeRow(node repo.ListNodesRow) models.Node {
	var lastSeen *time.Time
	if node.LastSeenAt.Valid {
		lastSeen = &node.LastSeenAt.Time
	}
	var lastPolicyCheckAt *time.Time
	if node.LastPolicyCheckAt.Valid {
		lastPolicyCheckAt = &node.LastPolicyCheckAt.Time
	}

	return models.Node{
		ID:                node.ID,
		ResourceID:        node.Uuid.String(),
		UUID:              node.Uuid.String(),
		NodeKey:           node.NodeKey.String(),
		HostIdentifier:    node.HostIdentifier,
		Hostname:          node.Hostname,
		Platform:          node.Platform,
		OSName:            node.OsName,
		OSVersion:         node.OsVersion,
		OSQueryVersion:    node.OsqueryVersion,
		HardwareSerial:    node.HardwareSerial,
		EnrolledAt:        node.EnrolledAt,
		LastSeenAt:        lastSeen,
		LastPolicyCheckAt: lastPolicyCheckAt,
		CreatedAt:         node.CreatedAt,
		UpdatedAt:         node.UpdatedAt,
	}
}

func toModelQuery(query repo.Query) models.Query {
	return models.Query{
		ID:          query.ID,
		UUID:        query.Uuid.String(),
		Name:        query.Name,
		SQL:         query.Sql,
		Title:       query.Name,
		Query:       query.Sql,
		IsSystem:    query.IsSystem,
		Description: query.Description,
		CreatedAt:   query.CreatedAt,
		UpdatedAt:   query.UpdatedAt,
	}
}

func toModelQueryRow(query repo.ListQueriesRow) models.Query {
	return models.Query{
		ID:          query.ID,
		UUID:        query.Uuid.String(),
		Name:        query.Name,
		SQL:         query.Sql,
		Title:       query.Name,
		Query:       query.Sql,
		IsSystem:    query.IsSystem,
		Description: query.Description,
		CreatedAt:   query.CreatedAt,
		UpdatedAt:   query.UpdatedAt,
	}
}

func toModelSchedule(schedule repo.Schedule, query models.Query) models.Schedule {
	return scheduleFromParts(scheduleParts{
		id:         schedule.ID,
		uuid:       schedule.Uuid.String(),
		name:       schedule.Name,
		interval:   int(schedule.IntervalSeconds),
		platform:   schedule.Platform,
		version:    schedule.Version,
		shard:      int(schedule.Shard),
		denylist:   schedule.Denylist,
		removed:    schedule.Removed,
		snapshot:   schedule.Snapshot,
		enabled:    schedule.Enabled,
		isSystem:   schedule.IsSystem,
		sqlVersion: int(schedule.SqlVersion),
		createdAt:  schedule.CreatedAt,
		updatedAt:  schedule.UpdatedAt,
		query:      query,
	})
}

func toModelScheduleWithQueryRow(row repo.GetScheduleWithQueryByUUIDRow) models.Schedule {
	query := models.Query{
		ID:          row.QueryIDValue,
		UUID:        row.QueryUuid.String(),
		Name:        row.QueryName,
		SQL:         row.QuerySql,
		Title:       row.QueryName,
		Query:       row.QuerySql,
		IsSystem:    row.QueryIsSystem,
		Description: row.QueryDescription,
		CreatedAt:   row.QueryCreatedAt,
		UpdatedAt:   row.QueryUpdatedAt,
	}

	return scheduleFromParts(scheduleParts{
		id:         row.ID,
		uuid:       row.Uuid.String(),
		name:       row.Name,
		interval:   int(row.IntervalSeconds),
		platform:   row.Platform,
		version:    row.Version,
		shard:      int(row.Shard),
		denylist:   row.Denylist,
		removed:    row.Removed,
		snapshot:   row.Snapshot,
		enabled:    row.Enabled,
		isSystem:   row.IsSystem,
		sqlVersion: int(row.SqlVersion),
		createdAt:  row.CreatedAt,
		updatedAt:  row.UpdatedAt,
		query:      query,
	})
}

func toModelScheduleRow(row repo.ListSchedulesWithQueriesRow) models.Schedule {
	query := models.Query{
		ID:          row.QueryIDValue,
		UUID:        row.QueryUuid.String(),
		Name:        row.QueryName,
		SQL:         row.QuerySql,
		Title:       row.QueryName,
		Query:       row.QuerySql,
		IsSystem:    row.QueryIsSystem,
		Description: row.QueryDescription,
		CreatedAt:   row.QueryCreatedAt,
		UpdatedAt:   row.QueryUpdatedAt,
	}

	return scheduleFromParts(scheduleParts{
		id:         row.ID,
		uuid:       row.Uuid.String(),
		name:       row.Name,
		interval:   int(row.IntervalSeconds),
		platform:   row.Platform,
		version:    row.Version,
		shard:      int(row.Shard),
		denylist:   row.Denylist,
		removed:    row.Removed,
		snapshot:   row.Snapshot,
		enabled:    row.Enabled,
		isSystem:   row.IsSystem,
		sqlVersion: int(row.SqlVersion),
		createdAt:  row.CreatedAt,
		updatedAt:  row.UpdatedAt,
		query:      query,
	})
}

func toModelEnabledScheduleForNodeRow(row repo.ListEnabledSchedulesForNodeRow) models.Schedule {
	query := models.Query{
		ID:          row.QueryIDValue,
		UUID:        row.QueryUuid.String(),
		Name:        row.QueryName,
		SQL:         row.QuerySql,
		Title:       row.QueryName,
		Query:       row.QuerySql,
		IsSystem:    row.QueryIsSystem,
		Description: row.QueryDescription,
		CreatedAt:   row.QueryCreatedAt,
		UpdatedAt:   row.QueryUpdatedAt,
	}

	return scheduleFromParts(scheduleParts{
		id:         row.ID,
		uuid:       row.Uuid.String(),
		name:       row.Name,
		interval:   int(row.IntervalSeconds),
		platform:   row.Platform,
		version:    row.Version,
		shard:      int(row.Shard),
		denylist:   row.Denylist,
		removed:    row.Removed,
		snapshot:   row.Snapshot,
		enabled:    row.Enabled,
		isSystem:   row.IsSystem,
		sqlVersion: int(row.SqlVersion),
		createdAt:  row.CreatedAt,
		updatedAt:  row.UpdatedAt,
		query:      query,
	})
}

func toModelEnabledScheduleRow(row repo.ListEnabledSchedulesWithQueriesRow) models.Schedule {
	query := models.Query{
		ID:          row.QueryIDValue,
		UUID:        row.QueryUuid.String(),
		Name:        row.QueryName,
		SQL:         row.QuerySql,
		Title:       row.QueryName,
		Query:       row.QuerySql,
		IsSystem:    row.QueryIsSystem,
		Description: row.QueryDescription,
		CreatedAt:   row.QueryCreatedAt,
		UpdatedAt:   row.QueryUpdatedAt,
	}

	return scheduleFromParts(scheduleParts{
		id:         row.ID,
		uuid:       row.Uuid.String(),
		name:       row.Name,
		interval:   int(row.IntervalSeconds),
		platform:   row.Platform,
		version:    row.Version,
		shard:      int(row.Shard),
		denylist:   row.Denylist,
		removed:    row.Removed,
		snapshot:   row.Snapshot,
		enabled:    row.Enabled,
		isSystem:   row.IsSystem,
		sqlVersion: int(row.SqlVersion),
		createdAt:  row.CreatedAt,
		updatedAt:  row.UpdatedAt,
		query:      query,
	})
}

// scheduleParts collects the fields shared across every sqlc schedule row
// shape so model assembly lives in one place.
type scheduleParts struct {
	id         int64
	uuid       string
	name       string
	interval   int
	platform   string
	version    string
	shard      int
	denylist   bool
	removed    bool
	snapshot   bool
	enabled    bool
	isSystem   bool
	sqlVersion int
	createdAt  time.Time
	updatedAt  time.Time
	query      models.Query
}

func scheduleFromParts(parts scheduleParts) models.Schedule {
	return models.Schedule{
		ID:              parts.id,
		UUID:            parts.uuid,
		QueryID:         parts.query.UUID,
		Query:           parts.query,
		Name:            parts.name,
		Title:           parts.name,
		VersionedName:   versionedScheduleName(parts.name, parts.sqlVersion),
		SQLVersion:      parts.sqlVersion,
		IntervalSeconds: parts.interval,
		Interval:        parts.interval,
		Removed:         parts.removed,
		Snapshot:        parts.snapshot,
		Platform:        parts.platform,
		Version:         parts.version,
		Shard:           parts.shard,
		Enabled:         parts.enabled,
		IsSystem:        parts.isSystem,
		Denylist:        parts.denylist,
		CreatedAt:       parts.createdAt,
		UpdatedAt:       parts.updatedAt,
	}
}

func versionedScheduleName(name string, sqlVersion int) string {
	if sqlVersion <= 0 {
		sqlVersion = 1
	}
	return fmt.Sprintf("%s/v%d", name, sqlVersion)
}

func toModelPolicy(policy repo.Policy) models.Policy {
	return models.Policy{
		ID:                policy.ID,
		ResourceID:        policy.Uuid.String(),
		UUID:              policy.Uuid.String(),
		Name:              policy.Name,
		Title:             policy.Name,
		Query:             policy.Query,
		Description:       policy.Description,
		Resolution:        policy.Resolution,
		Platform:          policy.Platform,
		Enabled:           policy.Enabled,
		IsSystem:          policy.IsSystem,
		TargetAllMachines: true,
		CreatedAt:         policy.CreatedAt,
		UpdatedAt:         policy.UpdatedAt,
	}
}

func toModelPolicyRow(row repo.ListPoliciesWithCountsRow) models.Policy {
	return models.Policy{
		ID:                 row.ID,
		ResourceID:         row.Uuid.String(),
		UUID:               row.Uuid.String(),
		Name:               row.Name,
		Title:              row.Name,
		Query:              row.Query,
		Description:        row.Description,
		Resolution:         row.Resolution,
		Platform:           row.Platform,
		Enabled:            row.Enabled,
		IsSystem:           row.IsSystem,
		TargetAllMachines:  true,
		PassingCount:       int(row.PassingCount),
		FailingCount:       int(row.FailingCount),
		UnknownCount:       int(row.UnknownCount),
		LastCountUpdatedAt: timePtrFromValue(row.LastCountUpdatedAt),
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}
}

func toModelPolicyPosture(row repo.ListPoliciesForNodeRow) models.PolicyPosture {
	return models.PolicyPosture{
		Policy: models.Policy{
			ID:                row.ID,
			ResourceID:        row.Uuid.String(),
			UUID:              row.Uuid.String(),
			Name:              row.Name,
			Title:             row.Name,
			Query:             row.Query,
			Description:       row.Description,
			Resolution:        row.Resolution,
			Platform:          row.Platform,
			Enabled:           row.Enabled,
			IsSystem:          row.IsSystem,
			TargetAllMachines: true,
			CreatedAt:         row.CreatedAt,
			UpdatedAt:         row.UpdatedAt,
		},
		Response:  row.Response,
		CheckedAt: timePtrFromNull(row.CheckedAt),
		LastError: row.LastError.String,
		Stale:     row.Stale,
	}
}

func toModelPolicyMachine(row repo.ListNodesByPolicyResponseRow) models.PolicyMachine {
	lastError := ""
	if row.LastError.Valid {
		lastError = row.LastError.String
	}

	return models.PolicyMachine{
		Node: models.Node{
			ID:                row.ID,
			ResourceID:        row.Uuid.String(),
			UUID:              row.Uuid.String(),
			NodeKey:           row.NodeKey.String(),
			HostIdentifier:    row.HostIdentifier,
			Hostname:          row.Hostname,
			Platform:          row.Platform,
			OSName:            row.OsName,
			OSVersion:         row.OsVersion,
			OSQueryVersion:    row.OsqueryVersion,
			HardwareSerial:    row.HardwareSerial,
			EnrolledAt:        row.EnrolledAt,
			LastSeenAt:        timePtrFromNull(row.LastSeenAt),
			LastPolicyCheckAt: timePtrFromNull(row.LastPolicyCheckAt),
			Groups:            nil,
			CreatedAt:         row.CreatedAt,
			UpdatedAt:         row.UpdatedAt,
		},
		Response:  row.Response,
		CheckedAt: timePtrFromNull(row.CheckedAt),
		LastError: lastError,
		Stale:     row.Stale,
	}
}

func toModelGroup(group repo.Group) models.Group {
	return models.Group{
		ID:          group.ID,
		ResourceID:  group.Uuid.String(),
		UUID:        group.Uuid.String(),
		Name:        group.Name,
		Description: group.Description,
		CreatedAt:   group.CreatedAt,
		UpdatedAt:   group.UpdatedAt,
	}
}

func toModelGroupRow(row repo.ListGroupsWithCountsRow) models.Group {
	return models.Group{
		ID:           row.ID,
		ResourceID:   row.Uuid.String(),
		UUID:         row.Uuid.String(),
		Name:         row.Name,
		Description:  row.Description,
		MachineCount: int(row.MachineCount),
		PolicyCount:  int(row.PolicyCount),
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}

func toModelGroupCountRow(row repo.GetGroupWithCountsByUUIDRow) models.Group {
	return models.Group{
		ID:           row.ID,
		ResourceID:   row.Uuid.String(),
		UUID:         row.Uuid.String(),
		Name:         row.Name,
		Description:  row.Description,
		MachineCount: int(row.MachineCount),
		PolicyCount:  int(row.PolicyCount),
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}

func toModelNodeFromGroupRow(row repo.ListNodesByGroupRow) models.Node {
	return models.Node{
		ID:                row.ID,
		ResourceID:        row.Uuid.String(),
		UUID:              row.Uuid.String(),
		NodeKey:           row.NodeKey.String(),
		HostIdentifier:    row.HostIdentifier,
		Hostname:          row.Hostname,
		Platform:          row.Platform,
		OSName:            row.OsName,
		OSVersion:         row.OsVersion,
		OSQueryVersion:    row.OsqueryVersion,
		HardwareSerial:    row.HardwareSerial,
		EnrolledAt:        row.EnrolledAt,
		LastSeenAt:        timePtrFromNull(row.LastSeenAt),
		LastPolicyCheckAt: timePtrFromNull(row.LastPolicyCheckAt),
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
}

func timePtrFromNull(value sql.NullTime) *time.Time {
	if value.Valid {
		return &value.Time
	}
	return nil
}

func timePtrFromValue(value interface{}) *time.Time {
	switch v := value.(type) {
	case nil:
		return nil
	case time.Time:
		return &v
	case *time.Time:
		return v
	default:
		return nil
	}
}
