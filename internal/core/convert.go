package core

import (
	"database/sql"
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
	return models.Schedule{
		ID:              schedule.ID,
		UUID:            schedule.Uuid.String(),
		QueryID:         query.UUID,
		Query:           query,
		Name:            schedule.Name,
		Title:           schedule.Name,
		IntervalSeconds: int(schedule.IntervalSeconds),
		Interval:        int(schedule.IntervalSeconds),
		Removed:         schedule.Removed,
		Snapshot:        schedule.Snapshot,
		Platform:        schedule.Platform,
		Version:         schedule.Version,
		Shard:           int(schedule.Shard),
		Enabled:         schedule.Enabled,
		IsSystem:        schedule.IsSystem,
		Denylist:        schedule.Denylist,
		CreatedAt:       schedule.CreatedAt,
		UpdatedAt:       schedule.UpdatedAt,
	}
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

	return scheduleFromParts(
		row.ID,
		row.Uuid.String(),
		row.Name,
		int(row.IntervalSeconds),
		row.Platform,
		row.Version,
		int(row.Shard),
		row.Denylist,
		row.Removed,
		row.Snapshot,
		row.Enabled,
		row.IsSystem,
		row.CreatedAt,
		row.UpdatedAt,
		query,
	)
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

	return scheduleFromParts(
		row.ID,
		row.Uuid.String(),
		row.Name,
		int(row.IntervalSeconds),
		row.Platform,
		row.Version,
		int(row.Shard),
		row.Denylist,
		row.Removed,
		row.Snapshot,
		row.Enabled,
		row.IsSystem,
		row.CreatedAt,
		row.UpdatedAt,
		query,
	)
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

	return scheduleFromParts(
		row.ID,
		row.Uuid.String(),
		row.Name,
		int(row.IntervalSeconds),
		row.Platform,
		row.Version,
		int(row.Shard),
		row.Denylist,
		row.Removed,
		row.Snapshot,
		row.Enabled,
		row.IsSystem,
		row.CreatedAt,
		row.UpdatedAt,
		query,
	)
}

func scheduleFromParts(id int64, uuid, name string, interval int, platform, version string, shard int, denylist, removed, snapshot, enabled, isSystem bool, createdAt, updatedAt time.Time, query models.Query) models.Schedule {
	return models.Schedule{
		ID:              id,
		UUID:            uuid,
		QueryID:         query.UUID,
		Query:           query,
		Name:            name,
		Title:           name,
		IntervalSeconds: interval,
		Interval:        interval,
		Removed:         removed,
		Snapshot:        snapshot,
		Platform:        platform,
		Version:         version,
		Shard:           shard,
		Enabled:         enabled,
		IsSystem:        isSystem,
		Denylist:        denylist,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
	}
}

func toModelPolicy(policy repo.Policy) models.Policy {
	return models.Policy{
		ID:          policy.ID,
		ResourceID:  policy.Uuid.String(),
		UUID:        policy.Uuid.String(),
		Name:        policy.Name,
		Title:       policy.Name,
		Query:       policy.Query,
		Description: policy.Description,
		Resolution:  policy.Resolution,
		Platform:    policy.Platform,
		Enabled:     policy.Enabled,
		IsSystem:    policy.IsSystem,
		CreatedAt:   policy.CreatedAt,
		UpdatedAt:   policy.UpdatedAt,
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
			ID:          row.ID,
			ResourceID:  row.Uuid.String(),
			UUID:        row.Uuid.String(),
			Name:        row.Name,
			Title:       row.Name,
			Query:       row.Query,
			Description: row.Description,
			Resolution:  row.Resolution,
			Platform:    row.Platform,
			Enabled:     row.Enabled,
			IsSystem:    row.IsSystem,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
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
			CreatedAt:         row.CreatedAt,
			UpdatedAt:         row.UpdatedAt,
		},
		Response:  row.Response,
		CheckedAt: timePtrFromNull(row.CheckedAt),
		LastError: lastError,
		Stale:     row.Stale,
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
