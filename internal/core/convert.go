package core

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/cvhariharan/checkpost/internal/repo"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
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

func toModelSchedule(schedule repo.Schedule) models.Schedule {
	return scheduleFromParts(scheduleParts{
		id:          schedule.ID,
		uuid:        schedule.Uuid.String(),
		name:        schedule.Name,
		sql:         schedule.Sql,
		description: schedule.Description,
		interval:    int(schedule.IntervalSeconds),
		platform:    schedule.Platform,
		version:     schedule.Version,
		shard:       int(schedule.Shard),
		denylist:    schedule.Denylist,
		removed:     schedule.Removed,
		snapshot:    schedule.Snapshot,
		enabled:     schedule.Enabled,
		isSystem:    schedule.IsSystem,
		sqlVersion:  int(schedule.SqlVersion),
		createdAt:   schedule.CreatedAt,
		updatedAt:   schedule.UpdatedAt,
	})
}

func toModelScheduleRow(row repo.ListSchedulesRow) models.Schedule {
	return scheduleFromParts(scheduleParts{
		id:          row.ID,
		uuid:        row.Uuid.String(),
		name:        row.Name,
		sql:         row.Sql,
		description: row.Description,
		interval:    int(row.IntervalSeconds),
		platform:    row.Platform,
		version:     row.Version,
		shard:       int(row.Shard),
		denylist:    row.Denylist,
		removed:     row.Removed,
		snapshot:    row.Snapshot,
		enabled:     row.Enabled,
		isSystem:    row.IsSystem,
		sqlVersion:  int(row.SqlVersion),
		createdAt:   row.CreatedAt,
		updatedAt:   row.UpdatedAt,
	})
}

// scheduleParts collects the fields shared across every sqlc schedule row
// shape so model assembly lives in one place.
type scheduleParts struct {
	id          int64
	uuid        string
	name        string
	sql         string
	description string
	interval    int
	platform    string
	version     string
	shard       int
	denylist    bool
	removed     bool
	snapshot    bool
	enabled     bool
	isSystem    bool
	sqlVersion  int
	createdAt   time.Time
	updatedAt   time.Time
}

func scheduleFromParts(parts scheduleParts) models.Schedule {
	return models.Schedule{
		ID:              parts.id,
		UUID:            parts.uuid,
		Name:            parts.name,
		Title:           parts.name,
		SQL:             parts.sql,
		Description:     parts.description,
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

func toModelPolicyPage(rows []repo.ListPoliciesWithCountsRow) ([]models.Policy, int) {
	out := make([]models.Policy, 0, len(rows))
	indexByID := make(map[int64]int, len(rows))
	totalCount := 0

	for _, row := range rows {
		totalCount = int(row.TotalCount)

		index, ok := indexByID[row.ID]
		if !ok {
			policy := toModelPolicyRow(row)
			policy.Groups = nil
			policy.TargetAllMachines = true
			out = append(out, policy)
			index = len(out) - 1
			indexByID[row.ID] = index
		}

		group, ok := toModelGroupFromPolicyRow(row)
		if !ok {
			continue
		}
		out[index].Groups = append(out[index].Groups, group)
		out[index].TargetAllMachines = false
	}

	return out, totalCount
}

func toModelGroupFromPolicyRow(row repo.ListPoliciesWithCountsRow) (models.Group, bool) {
	if !row.GroupID.Valid || !row.GroupUuid.Valid {
		return models.Group{}, false
	}

	group := models.Group{
		ID:         row.GroupID.Int64,
		ResourceID: row.GroupUuid.UUID.String(),
		UUID:       row.GroupUuid.UUID.String(),
	}
	if row.GroupName.Valid {
		group.Name = row.GroupName.String
	}
	if row.GroupDescription.Valid {
		group.Description = row.GroupDescription.String
	}
	if row.GroupCreatedAt.Valid {
		group.CreatedAt = row.GroupCreatedAt.Time
	}
	if row.GroupUpdatedAt.Valid {
		group.UpdatedAt = row.GroupUpdatedAt.Time
	}
	return group, true
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

func toModelDeviceOwner(owner repo.DeviceOwner) models.DeviceOwner {
	return models.DeviceOwner{
		ID:          owner.ID,
		ResourceID:  owner.Uuid.String(),
		UUID:        owner.Uuid.String(),
		DisplayName: owner.DisplayName,
		Email:       owner.Email,
		ExternalID:  owner.ExternalID,
		Department:  owner.Department,
		Title:       owner.Title,
		Phone:       owner.Phone,
		Notes:       owner.Notes,
		CreatedAt:   owner.CreatedAt,
		UpdatedAt:   owner.UpdatedAt,
	}
}

func toModelDeviceOwnerRow(row repo.ListDeviceOwnersWithCountsRow) models.DeviceOwner {
	return models.DeviceOwner{
		ID:           row.ID,
		ResourceID:   row.Uuid.String(),
		UUID:         row.Uuid.String(),
		DisplayName:  row.DisplayName,
		Email:        row.Email,
		ExternalID:   row.ExternalID,
		Department:   row.Department,
		Title:        row.Title,
		Phone:        row.Phone,
		Notes:        row.Notes,
		MachineCount: int(row.MachineCount),
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}

func toModelDeviceOwnerCountRow(row repo.GetDeviceOwnerWithCountsByUUIDRow) models.DeviceOwner {
	return models.DeviceOwner{
		ID:           row.ID,
		ResourceID:   row.Uuid.String(),
		UUID:         row.Uuid.String(),
		DisplayName:  row.DisplayName,
		Email:        row.Email,
		ExternalID:   row.ExternalID,
		Department:   row.Department,
		Title:        row.Title,
		Phone:        row.Phone,
		Notes:        row.Notes,
		MachineCount: int(row.MachineCount),
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}

type nodeInventoryParts struct {
	internalTrackingID string
	notes              string
	createdAt          time.Time
	updatedAt          time.Time
	ownerID            sql.NullInt64
	ownerUUID          uuid.NullUUID
	ownerDisplayName   sql.NullString
	ownerEmail         sql.NullString
	ownerExternalID    sql.NullString
	ownerDepartment    sql.NullString
	ownerTitle         sql.NullString
	ownerPhone         sql.NullString
	ownerNotes         sql.NullString
	ownerCreatedAt     sql.NullTime
	ownerUpdatedAt     sql.NullTime
}

func toModelNodeInventory(row repo.GetNodeInventoryByNodeUUIDRow) models.NodeInventory {
	return nodeInventoryFromParts(nodeInventoryParts{
		internalTrackingID: row.InternalTrackingID,
		notes:              row.Notes,
		createdAt:          row.CreatedAt,
		updatedAt:          row.UpdatedAt,
		ownerID:            row.OwnerDbID,
		ownerUUID:          row.OwnerUuid,
		ownerDisplayName:   row.OwnerDisplayName,
		ownerEmail:         row.OwnerEmail,
		ownerExternalID:    row.OwnerExternalID,
		ownerDepartment:    row.OwnerDepartment,
		ownerTitle:         row.OwnerTitle,
		ownerPhone:         row.OwnerPhone,
		ownerNotes:         row.OwnerNotes,
		ownerCreatedAt:     row.OwnerCreatedAt,
		ownerUpdatedAt:     row.OwnerUpdatedAt,
	})
}

func toModelNodeInventoryListRow(row repo.ListNodeInventoriesByNodeUUIDsRow) models.NodeInventory {
	return nodeInventoryFromParts(nodeInventoryParts{
		internalTrackingID: row.InternalTrackingID,
		notes:              row.Notes,
		createdAt:          row.CreatedAt,
		updatedAt:          row.UpdatedAt,
		ownerID:            row.OwnerDbID,
		ownerUUID:          row.OwnerUuid,
		ownerDisplayName:   row.OwnerDisplayName,
		ownerEmail:         row.OwnerEmail,
		ownerExternalID:    row.OwnerExternalID,
		ownerDepartment:    row.OwnerDepartment,
		ownerTitle:         row.OwnerTitle,
		ownerPhone:         row.OwnerPhone,
		ownerNotes:         row.OwnerNotes,
		ownerCreatedAt:     row.OwnerCreatedAt,
		ownerUpdatedAt:     row.OwnerUpdatedAt,
	})
}

func nodeInventoryFromParts(parts nodeInventoryParts) models.NodeInventory {
	inventory := models.NodeInventory{
		InternalTrackingID: parts.internalTrackingID,
		Notes:              parts.notes,
		CreatedAt:          parts.createdAt,
		UpdatedAt:          parts.updatedAt,
	}
	if parts.ownerID.Valid && parts.ownerUUID.Valid {
		owner := models.DeviceOwner{
			ID:          parts.ownerID.Int64,
			ResourceID:  parts.ownerUUID.UUID.String(),
			UUID:        parts.ownerUUID.UUID.String(),
			DisplayName: parts.ownerDisplayName.String,
			Email:       parts.ownerEmail.String,
			ExternalID:  parts.ownerExternalID.String,
			Department:  parts.ownerDepartment.String,
			Title:       parts.ownerTitle.String,
			Phone:       parts.ownerPhone.String,
			Notes:       parts.ownerNotes.String,
			CreatedAt:   parts.ownerCreatedAt.Time,
			UpdatedAt:   parts.ownerUpdatedAt.Time,
		}
		inventory.Owner = &owner
	}
	return inventory
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

func toModelNodeFromOwnerRow(row repo.ListNodesByOwnerRow) models.Node {
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

func toModelUser(u repo.User) models.User {
	return models.User{
		ID:          u.ID,
		UUID:        u.Uuid.String(),
		Username:    u.Username,
		Name:        u.Name,
		Email:       u.Email,
		LoginType:   u.LoginType,
		Disabled:    u.Disabled,
		LastLoginAt: timePtrFromNull(u.LastLoginAt),
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}

func toModelUserGroup(g repo.UserGroup) models.UserGroup {
	return models.UserGroup{
		ID:             g.ID,
		UUID:           g.Uuid.String(),
		Name:           g.Name,
		Description:    g.Description,
		OIDCClaimValue: g.OidcClaimValue,
		CreatedAt:      g.CreatedAt,
		UpdatedAt:      g.UpdatedAt,
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

func toModelMachineQueryResult(row repo.MachineQueryResult) models.MachineQueryResult {
	timestamp := row.CreatedAt
	if row.CompletedAt.Valid {
		timestamp = row.CompletedAt.Time
	}

	return models.MachineQueryResult{
		ID:        row.Uuid.String(),
		Query:     row.Query,
		Status:    row.Status,
		Timestamp: timestamp,
		Results:   decodeMachineQueryResults(row.Results),
		Error:     row.Error,
	}
}

func toModelMachineQueryResultRow(row repo.ListMachineQueryResultsByNodeUUIDRow) models.MachineQueryResult {
	timestamp := row.CreatedAt
	if row.CompletedAt.Valid {
		timestamp = row.CompletedAt.Time
	}

	return models.MachineQueryResult{
		ID:        row.Uuid.String(),
		Query:     row.Query,
		Status:    row.Status,
		Timestamp: timestamp,
		Results:   decodeMachineQueryResults(row.Results),
		Error:     row.Error,
	}
}

func decodeMachineQueryResults(raw pqtype.NullRawMessage) interface{} {
	if !raw.Valid || len(raw.RawMessage) == 0 {
		return nil
	}

	var out interface{}
	if err := json.Unmarshal(raw.RawMessage, &out); err != nil {
		return string(raw.RawMessage)
	}
	return out
}

func toModelAlertTarget(t repo.AlertTarget) models.AlertTarget {
	return models.AlertTarget{
		UUID:      t.Uuid.String(),
		Name:      t.Name,
		Type:      t.Type,
		Config:    t.Config,
		Enabled:   t.Enabled,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

func toModelAPIToken(row repo.ApiToken) models.APIToken {
	return models.APIToken{
		UUID:       row.Uuid.String(),
		Name:       row.Name,
		Source:     row.Source,
		ExpiresAt:  timePtrFromNull(row.ExpiresAt),
		LastUsedAt: timePtrFromNull(row.LastUsedAt),
		RevokedAt:  timePtrFromNull(row.RevokedAt),
		CreatedAt:  row.CreatedAt,
	}
}
