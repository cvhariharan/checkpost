package models

import "github.com/cvhariharan/watcher/internal/repo"

func YaraSignatureSourceFromRow(row repo.ListYaraSignatureSourcesRow) YaraSignatureSource {
	out := YaraSignatureSource{
		ID:        row.Uuid.String(),
		UUID:      row.Uuid.String(),
		URL:       row.Url,
		Label:     row.Label,
		Enabled:   row.Enabled,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
	if row.GroupUuid.Valid {
		out.GroupID = row.GroupUuid.UUID.String()
	}
	if row.GroupName.Valid {
		out.GroupName = row.GroupName.String
	}
	return out
}

func YaraScanFromRow(row repo.GetYaraScanByUUIDRow) YaraScan {
	out := YaraScan{
		ID:             row.Uuid.String(),
		UUID:           row.Uuid.String(),
		Path:           row.Path,
		Status:         row.Status,
		TargetCount:    int(row.TargetCount),
		CompletedCount: int(row.CompletedCount),
		MatchCount:     int(row.MatchCount),
		Error:          row.Error,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
	if row.GroupUuid.Valid {
		out.GroupID = row.GroupUuid.UUID.String()
	}
	if row.GroupName.Valid {
		out.GroupName = row.GroupName.String
	}
	if row.CompletedAt.Valid {
		out.CompletedAt = &row.CompletedAt.Time
	}
	return out
}

func YaraScanFromListRow(row repo.ListYaraScansRow) YaraScan {
	out := YaraScan{
		ID:             row.Uuid.String(),
		UUID:           row.Uuid.String(),
		Path:           row.Path,
		Status:         row.Status,
		TargetCount:    int(row.TargetCount),
		CompletedCount: int(row.CompletedCount),
		MatchCount:     int(row.MatchCount),
		Error:          row.Error,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
	if row.GroupUuid.Valid {
		out.GroupID = row.GroupUuid.UUID.String()
	}
	if row.GroupName.Valid {
		out.GroupName = row.GroupName.String
	}
	if row.CompletedAt.Valid {
		out.CompletedAt = &row.CompletedAt.Time
	}
	return out
}

func YaraScanMatchFromRow(row repo.ListYaraScanMatchesRow) YaraScanMatch {
	return YaraScanMatch{
		MachineUUID: row.NodeUuid.String(),
		Hostname:    row.Hostname,
		Path:        row.Path,
		Matches:     row.Matches,
		Count:       int(row.Count),
		CreatedAt:   row.CreatedAt,
	}
}

func YaraScanTargetFromRow(row repo.ListYaraScanTargetsRow) YaraScanTarget {
	target := YaraScanTarget{
		MachineUUID: row.NodeUuid.String(),
		Hostname:    row.Hostname,
		Status:      row.Status,
		Error:       row.Error,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
	if row.DispatchedAt.Valid {
		target.DispatchedAt = &row.DispatchedAt.Time
	}
	if row.CompletedAt.Valid {
		target.CompletedAt = &row.CompletedAt.Time
	}
	return target
}
