package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/cvhariharan/checkpost/internal/repo"
	"github.com/cvhariharan/checkpost/internal/results"
	"github.com/google/uuid"
)

// MaxQueryRunHosts caps the number of hosts a single query run may target so a
// broad selection can't fan out into an unbounded number of executions.
const MaxQueryRunHosts = 1000

var (
	// ErrNoQueryTargets is returned when the selected targets resolve to no hosts.
	ErrNoQueryTargets = errors.New("no hosts matched the selected targets")
	// ErrTooManyQueryTargets is returned when the resolved host set exceeds MaxQueryRunHosts.
	ErrTooManyQueryTargets = fmt.Errorf("too many hosts targeted (max %d)", MaxQueryRunHosts)
)

// ResolveQueryTargets resolves the host/group/platform selectors into a
// deduplicated, deterministically ordered set of node IDs.
func (c *Core) ResolveQueryTargets(ctx context.Context, targets models.QueryTargets) ([]int64, error) {
	idSet := make(map[int64]struct{})

	if len(targets.HostIDs) > 0 {
		hostUUIDs, err := parseUUIDs(targets.HostIDs)
		if err != nil {
			return nil, fmt.Errorf("parse host ids: %w", err)
		}
		ids, err := c.store.ListNodeIDsByUUIDs(ctx, hostUUIDs)
		if err != nil {
			return nil, fmt.Errorf("resolve host targets: %w", err)
		}
		for _, id := range ids {
			idSet[id] = struct{}{}
		}
	}

	if len(targets.GroupIDs) > 0 {
		groupUUIDs, err := parseUUIDs(targets.GroupIDs)
		if err != nil {
			return nil, fmt.Errorf("parse group ids: %w", err)
		}
		ids, err := c.store.ListNodeIDsByGroupUUIDs(ctx, groupUUIDs)
		if err != nil {
			return nil, fmt.Errorf("resolve group targets: %w", err)
		}
		for _, id := range ids {
			idSet[id] = struct{}{}
		}
	}

	if len(targets.Platforms) > 0 {
		platforms := make([]string, 0, len(targets.Platforms))
		for _, p := range targets.Platforms {
			platforms = append(platforms, strings.ToLower(strings.TrimSpace(p)))
		}
		ids, err := c.store.ListNodeIDsByPlatforms(ctx, platforms)
		if err != nil {
			return nil, fmt.Errorf("resolve platform targets: %w", err)
		}
		for _, id := range ids {
			idSet[id] = struct{}{}
		}
	}

	ids := make([]int64, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

// CreateQueryRun validates the query, resolves targets to a node set, and
// creates the run plus one pending ad-hoc execution per node in one transaction.
func (c *Core) CreateQueryRun(ctx context.Context, req models.QueryRunRequest) (models.QueryRun, error) {
	if c.sink == nil {
		return models.QueryRun{}, ErrResultsBackendDisabled
	}

	query := strings.TrimSpace(req.Query)
	if err := validateQuery(query); err != nil {
		return models.QueryRun{}, err
	}

	nodeIDs, err := c.ResolveQueryTargets(ctx, req.Targets)
	if err != nil {
		return models.QueryRun{}, err
	}
	if len(nodeIDs) == 0 {
		return models.QueryRun{}, ErrNoQueryTargets
	}
	if len(nodeIDs) > MaxQueryRunHosts {
		return models.QueryRun{}, ErrTooManyQueryTargets
	}

	targetsJSON, err := json.Marshal(req.Targets)
	if err != nil {
		return models.QueryRun{}, fmt.Errorf("marshal query targets: %w", err)
	}

	run, err := c.store.CreateQueryRunTx(ctx, repo.CreateQueryRunTxParams{
		Run: repo.CreateQueryRunParams{
			Uuid:      uuid.New(),
			Query:     query,
			Targets:   string(targetsJSON),
			CreatedBy: c.resolveCreatedBy(ctx, req.CreatedByUUID),
		},
		NodeIDs: nodeIDs,
	})
	if err != nil {
		return models.QueryRun{}, fmt.Errorf("create query run: %w", err)
	}

	return c.GetQueryRun(ctx, models.ResourceID{UUID: run.Uuid.String()})
}

// resolveCreatedBy maps the authenticated user's UUID to its row ID for audit.
// A missing or unresolvable user leaves created_by NULL rather than failing the run.
func (c *Core) resolveCreatedBy(ctx context.Context, userUUID string) sql.NullInt64 {
	if userUUID == "" {
		return sql.NullInt64{}
	}
	uid, err := uuid.Parse(userUUID)
	if err != nil {
		return sql.NullInt64{}
	}
	user, err := c.store.GetUserByUUID(ctx, uid)
	if err != nil {
		c.logger.Debug("query run created_by user not found", "user_uuid", userUUID, "error", err)
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: user.ID, Valid: true}
}

func (c *Core) ListQueryRuns(ctx context.Context, pageReq models.PageRequest) (models.Page[models.QueryRun], error) {
	countPerPage := pageReq.Count
	if countPerPage <= 0 {
		countPerPage = 10
	}
	page := pageReq.Page
	if page < 0 {
		page = 0
	}

	rows, err := c.store.ListQueryRuns(ctx, repo.ListQueryRunsParams{
		LimitCount:  int32(countPerPage),
		OffsetCount: int32(page * countPerPage),
	})
	if err != nil {
		return models.Page[models.QueryRun]{}, fmt.Errorf("list query runs: %w", err)
	}

	out := make([]models.QueryRun, 0, len(rows))
	totalCount := 0
	for _, row := range rows {
		out = append(out, toModelQueryRunListRow(row))
		totalCount = int(row.TotalCount)
	}
	return models.Page[models.QueryRun]{
		Items:      out,
		TotalCount: totalCount,
		PageCount:  pageCountFor(totalCount, countPerPage),
	}, nil
}

func (c *Core) GetQueryRun(ctx context.Context, req models.ResourceID) (models.QueryRun, error) {
	runUUID, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.QueryRun{}, fmt.Errorf("parse query run uuid: %w", err)
	}

	run, err := c.store.GetQueryRunByUUID(ctx, runUUID)
	if err != nil {
		return models.QueryRun{}, fmt.Errorf("get query run: %w", err)
	}

	hosts, err := c.store.ListMachineQueryResultsByRunUUID(ctx, runUUID)
	if err != nil {
		return models.QueryRun{}, fmt.Errorf("list query run hosts: %w", err)
	}

	return toModelQueryRun(run, hosts), nil
}

func (c *Core) ExportQueryRunResults(ctx context.Context, runID string, w io.Writer, format string) error {
	if format != "csv" {
		return ErrExportUnsupported
	}
	exp, err := c.resultExporter()
	if err != nil {
		return err
	}
	runUUID, err := uuid.Parse(runID)
	if err != nil {
		return fmt.Errorf("parse query run uuid: %w", err)
	}
	if _, err := c.store.GetQueryRunByUUID(ctx, runUUID); err != nil {
		return fmt.Errorf("get query run: %w", err)
	}
	hosts, err := c.store.ListMachineQueryResultsByRunUUID(ctx, runUUID)
	if err != nil {
		return fmt.Errorf("list query run hosts: %w", err)
	}

	var columns []string
	sources := make([]results.ExportSource, 0, len(hosts))
	for _, host := range hosts {
		if host.Status != "complete" || host.Error != "" || host.RowCount <= 0 {
			continue
		}
		// loadResultColumns returns a non-nil empty slice when a host has no
		// recorded schema, so guard on length (not nil) and keep looking until
		// a host yields columns
		if len(columns) == 0 {
			cols, err := c.loadResultColumns(ctx, host.Uuid, adHocSQLVersion)
			if err != nil {
				return err
			}
			columns = cols
		}
		sources = append(sources, results.ExportSource{
			SourceUUID: host.Uuid,
			SQLVersion: adHocSQLVersion,
			Hostname:   host.Hostname,
			NodeID:     host.NodeID,
		})
	}
	if len(sources) == 0 {
		return ErrResultNotReady
	}
	return exp.Export(ctx, w, results.ExportRequest{
		Format:      format,
		Snapshot:    true,
		Columns:     columns,
		Sources:     sources,
		IncludeHost: true,
	})
}

func (c *Core) DeleteQueryRun(ctx context.Context, req models.ResourceID) error {
	runUUID, err := uuid.Parse(req.UUID)
	if err != nil {
		return fmt.Errorf("parse query run uuid: %w", err)
	}

	queryUUIDs, err := c.store.ListMachineQueryUUIDsByRunUUID(ctx, runUUID)
	if err != nil {
		return fmt.Errorf("list query run results: %w", err)
	}

	rows, err := c.store.DeleteQueryRunByUUID(ctx, runUUID)
	if err != nil {
		return fmt.Errorf("delete query run: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	for _, queryUUID := range queryUUIDs {
		c.deleteResultSource(ctx, queryUUID)
	}
	return nil
}

func parseUUIDs(values []string) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, 0, len(values))
	for _, v := range values {
		parsed, err := uuid.Parse(v)
		if err != nil {
			return nil, fmt.Errorf("invalid uuid %q: %w", v, err)
		}
		out = append(out, parsed)
	}
	return out, nil
}
