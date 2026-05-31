package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/cvhariharan/watcher/internal/models"
	"github.com/cvhariharan/watcher/internal/repo"
	"github.com/google/uuid"
)

const (
	yaraQueryPrefix       = "yara:"
	yaraScanTargetTimeout = 5 * time.Minute
)

func (c *Core) ListYaraSignatureSources(ctx context.Context, pageReq models.PageRequest) (models.Page[models.YaraSignatureSource], error) {
	page, count := normalizePage(pageReq)
	rows, err := c.store.ListYaraSignatureSources(ctx, repo.ListYaraSignatureSourcesParams{
		LimitCount:  int32(count),
		OffsetCount: int32(page * count),
	})
	if err != nil {
		return models.Page[models.YaraSignatureSource]{}, fmt.Errorf("list yara signature sources: %w", err)
	}

	out := make([]models.YaraSignatureSource, 0, len(rows))
	totalCount := 0
	for _, row := range rows {
		out = append(out, models.YaraSignatureSourceFromRow(row))
		totalCount = int(row.TotalCount)
	}
	return models.Page[models.YaraSignatureSource]{
		Items:      out,
		TotalCount: totalCount,
		PageCount:  pageCountFor(totalCount, count),
	}, nil
}

func (c *Core) CreateYaraSignatureSource(ctx context.Context, req models.YaraSignatureSourceRequest) (models.YaraSignatureSource, error) {
	groupID, err := c.yaraSourceGroupID(ctx, req.GroupID)
	if err != nil {
		return models.YaraSignatureSource{}, err
	}
	created, err := c.store.CreateYaraSignatureSource(ctx, repo.CreateYaraSignatureSourceParams{
		GroupID: groupID,
		Url:     strings.TrimSpace(req.URL),
		Label:   strings.TrimSpace(req.Label),
		Enabled: req.Enabled,
	})
	if err != nil {
		return models.YaraSignatureSource{}, fmt.Errorf("create yara signature source: %w", err)
	}
	return c.getYaraSignatureSourceModel(ctx, created.Uuid)
}

func (c *Core) UpdateYaraSignatureSource(ctx context.Context, req models.YaraSignatureSourceRequest) (models.YaraSignatureSource, error) {
	sourceUUID, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.YaraSignatureSource{}, fmt.Errorf("parse yara source uuid: %w", err)
	}
	if err := validateYaraSourceURL(req.URL); err != nil {
		return models.YaraSignatureSource{}, fmt.Errorf("invalid yara source url: %w", err)
	}

	groupID, err := c.yaraSourceGroupID(ctx, req.GroupID)
	if err != nil {
		return models.YaraSignatureSource{}, err
	}
	updated, err := c.store.UpdateYaraSignatureSourceByUUID(ctx, repo.UpdateYaraSignatureSourceByUUIDParams{
		Uuid:    sourceUUID,
		GroupID: groupID,
		Url:     strings.TrimSpace(req.URL),
		Label:   strings.TrimSpace(req.Label),
		Enabled: req.Enabled,
	})
	if err != nil {
		return models.YaraSignatureSource{}, fmt.Errorf("update yara signature source: %w", err)
	}
	return c.getYaraSignatureSourceModel(ctx, updated.Uuid)
}

func (c *Core) DeleteYaraSignatureSource(ctx context.Context, req models.ResourceID) error {
	sourceUUID, err := uuid.Parse(req.UUID)
	if err != nil {
		return fmt.Errorf("parse yara source uuid: %w", err)
	}
	rows, err := c.store.DeleteYaraSignatureSourceByUUID(ctx, sourceUUID)
	if err != nil {
		return fmt.Errorf("delete yara signature source: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (c *Core) CreateYaraScan(ctx context.Context, req models.YaraScanRequest) (models.YaraScan, error) {
	paths, err := normalizeYaraPaths(req.Paths)
	if err != nil {
		return models.YaraScan{}, err
	}

	var groupID sql.NullInt64
	var groupUUID uuid.UUID
	if strings.TrimSpace(req.GroupID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(req.GroupID))
		if err != nil {
			return models.YaraScan{}, fmt.Errorf("parse group uuid: %w", err)
		}
		group, err := c.store.GetGroupByUUID(ctx, parsed)
		if err != nil {
			return models.YaraScan{}, fmt.Errorf("get group: %w", err)
		}
		groupID = sql.NullInt64{Int64: group.ID, Valid: true}
		groupUUID = parsed
	}

	var nodeIDs []int64
	if groupUUID == uuid.Nil {
		nodeIDs, err = c.store.ListAllNodeIDs(ctx)
	} else {
		nodeIDs, err = c.store.ListNodeIDsByGroupUUID(ctx, groupUUID)
	}
	if err != nil {
		return models.YaraScan{}, err
	}
	if len(nodeIDs) == 0 {
		return models.YaraScan{}, fmt.Errorf("%w: no machines match the selected group", ErrInvalidQuery)
	}

	ruleURLs, err := normalizeYaraRuleURLs(req.RuleURLs)
	if err != nil {
		return models.YaraScan{}, err
	}

	scan, err := c.store.CreateYaraScanTx(ctx, repo.CreateYaraScanTxParams{
		GroupID:  groupID,
		Paths:    paths,
		NodeIDs:  nodeIDs,
		RuleURLs: ruleURLs,
	})
	if err != nil {
		return models.YaraScan{}, fmt.Errorf("create yara scan: %w", err)
	}

	row, err := c.store.GetYaraScanByUUID(ctx, scan.Uuid)
	if err != nil {
		return models.YaraScan{}, fmt.Errorf("get yara scan: %w", err)
	}
	return models.YaraScanFromRow(row), nil
}

func (c *Core) ListYaraScans(ctx context.Context, pageReq models.PageRequest) (models.Page[models.YaraScan], error) {
	if err := c.expireStaleYaraTargets(ctx); err != nil {
		return models.Page[models.YaraScan]{}, err
	}
	page, count := normalizePage(pageReq)
	rows, err := c.store.ListYaraScans(ctx, repo.ListYaraScansParams{
		LimitCount:  int32(count),
		OffsetCount: int32(page * count),
	})
	if err != nil {
		return models.Page[models.YaraScan]{}, fmt.Errorf("list yara scans: %w", err)
	}
	out := make([]models.YaraScan, 0, len(rows))
	totalCount := 0
	for _, row := range rows {
		out = append(out, models.YaraScanFromListRow(row))
		totalCount = int(row.TotalCount)
	}
	return models.Page[models.YaraScan]{
		Items:      out,
		TotalCount: totalCount,
		PageCount:  pageCountFor(totalCount, count),
	}, nil
}

func normalizeYaraPaths(rawPaths []string) ([]string, error) {
	if len(rawPaths) == 0 {
		return nil, fmt.Errorf("%w: at least one path is required", ErrInvalidQuery)
	}
	return dedup(rawPaths, func(path string) (string, error) {
		if path == "" || strings.ContainsRune(path, '\x00') {
			return "", fmt.Errorf("%w: path is required", ErrInvalidQuery)
		}
		return path, nil
	})
}

func normalizeYaraRuleURLs(rawRuleURLs []string) ([]string, error) {
	if len(rawRuleURLs) == 0 {
		return nil, fmt.Errorf("%w: at least one YARA rule URL is required", ErrInvalidQuery)
	}
	return dedup(rawRuleURLs, func(ruleURL string) (string, error) {
		if ruleURL == "" {
			return "", fmt.Errorf("%w: YARA rule URL is required", ErrInvalidQuery)
		}
		parsed, err := url.Parse(ruleURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return "", fmt.Errorf("%w: YARA rule URL must be https", ErrInvalidQuery)
		}
		return strings.ToLower(ruleURL), nil
	})
}

// dedup and trim spaces in values in a string array
func dedup(values []string, keyFor func(string) (string, error)) ([]string, error) {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		key, err := keyFor(value)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out, nil
}

func (c *Core) GetYaraScan(ctx context.Context, req models.ResourceID) (models.YaraScan, error) {
	if err := c.expireStaleYaraTargets(ctx); err != nil {
		return models.YaraScan{}, err
	}
	scanUUID, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.YaraScan{}, fmt.Errorf("parse yara scan uuid: %w", err)
	}
	row, err := c.store.GetYaraScanByUUID(ctx, scanUUID)
	if err != nil {
		return models.YaraScan{}, fmt.Errorf("get yara scan: %w", err)
	}
	return models.YaraScanFromRow(row), nil
}

func (c *Core) ListYaraScanMatches(ctx context.Context, req models.ResourceID, pageReq models.PageRequest) (models.Page[models.YaraScanMatch], error) {
	if err := c.expireStaleYaraTargets(ctx); err != nil {
		return models.Page[models.YaraScanMatch]{}, err
	}
	scanUUID, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.Page[models.YaraScanMatch]{}, fmt.Errorf("parse yara scan uuid: %w", err)
	}
	page, count := normalizePage(pageReq)
	rows, err := c.store.ListYaraScanMatches(ctx, repo.ListYaraScanMatchesParams{
		ScanUuid:    scanUUID,
		LimitCount:  int32(count),
		OffsetCount: int32(page * count),
	})
	if err != nil {
		return models.Page[models.YaraScanMatch]{}, fmt.Errorf("list yara scan matches: %w", err)
	}
	out := make([]models.YaraScanMatch, 0, len(rows))
	totalCount := 0
	for _, row := range rows {
		out = append(out, models.YaraScanMatchFromRow(row))
		totalCount = int(row.TotalCount)
	}
	return models.Page[models.YaraScanMatch]{
		Items:      out,
		TotalCount: totalCount,
		PageCount:  pageCountFor(totalCount, count),
	}, nil
}

func (c *Core) ListYaraScanTargets(ctx context.Context, req models.ResourceID, pageReq models.PageRequest) (models.Page[models.YaraScanTarget], error) {
	if err := c.expireStaleYaraTargets(ctx); err != nil {
		return models.Page[models.YaraScanTarget]{}, err
	}
	scanUUID, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.Page[models.YaraScanTarget]{}, fmt.Errorf("parse yara scan uuid: %w", err)
	}
	page, count := normalizePage(pageReq)
	rows, err := c.store.ListYaraScanTargets(ctx, repo.ListYaraScanTargetsParams{
		ScanUuid:    scanUUID,
		LimitCount:  int32(count),
		OffsetCount: int32(page * count),
	})
	if err != nil {
		return models.Page[models.YaraScanTarget]{}, fmt.Errorf("list yara scan targets: %w", err)
	}
	out := make([]models.YaraScanTarget, 0, len(rows))
	totalCount := 0
	for _, row := range rows {
		out = append(out, models.YaraScanTargetFromRow(row))
		totalCount = int(row.TotalCount)
	}
	return models.Page[models.YaraScanTarget]{
		Items:      out,
		TotalCount: totalCount,
		PageCount:  pageCountFor(totalCount, count),
	}, nil
}

func (c *Core) PendingYaraQueries(ctx context.Context, node models.Node) (map[string]string, error) {
	if err := c.expireStaleYaraTargets(ctx); err != nil {
		return nil, err
	}
	pending, err := c.store.ListPendingYaraScanTargetsForNode(ctx, node.ID)
	if err != nil {
		return nil, fmt.Errorf("list pending yara scans: %w", err)
	}
	if len(pending) == 0 {
		return nil, nil
	}

	queries := make(map[string]string, len(pending))
	for _, item := range pending {
		if len(item.Paths) == 0 {
			return nil, fmt.Errorf("%w: no paths for scan", ErrInvalidQuery)
		}
		if len(item.RuleUrls) == 0 {
			return nil, fmt.Errorf("%w: no YARA rule URLs for scan", ErrInvalidQuery)
		}
		queries[yaraQueryPrefix+item.ScanUuid.String()] = yaraFileQuery(item.Paths, item.RuleUrls)
		if err := c.store.MarkYaraScanTargetDispatched(ctx, repo.MarkYaraScanTargetDispatchedParams{
			ScanID: item.ScanID,
			NodeID: node.ID,
		}); err != nil {
			return nil, fmt.Errorf("mark yara scan target dispatched: %w", err)
		}
		if err := c.store.MarkYaraScanRunning(ctx, item.ScanID); err != nil {
			return nil, fmt.Errorf("mark yara scan running: %w", err)
		}
	}
	return queries, nil
}

func yaraFileQuery(paths []string, ruleURLs []string) string {
	pathConditions := make([]string, 0, len(paths))
	for _, path := range paths {
		pathConditions = append(pathConditions, fmt.Sprintf("path LIKE '%s'", sqlString(strings.TrimSpace(path))))
	}
	parts := make([]string, 0, len(ruleURLs))
	for _, ruleURL := range ruleURLs {
		parts = append(parts, fmt.Sprintf(
			"SELECT path, matches, count, sigurl FROM yara_file WHERE (%s) AND sigurl = '%s' AND count > 0",
			strings.Join(pathConditions, " OR "),
			sqlString(strings.TrimSpace(ruleURL)),
		))
	}
	return strings.Join(parts, " UNION ALL ")
}

func (c *Core) completeYaraResult(ctx context.Context, node models.Node, queryID string, rows interface{}, errMsg string) (bool, error) {
	if !strings.HasPrefix(queryID, yaraQueryPrefix) {
		return false, nil
	}
	scanUUID, err := uuid.Parse(strings.TrimPrefix(queryID, yaraQueryPrefix))
	if err != nil {
		c.logger.Debug("ignoring malformed yara query id", "query_id", queryID, "error", err)
		return true, nil
	}
	scan, err := c.store.GetYaraScanByUUID(ctx, scanUUID)
	if err != nil {
		return true, fmt.Errorf("get yara scan: %w", err)
	}

	if errMsg != "" {
		if err := c.store.ErrorYaraScanTarget(ctx, repo.ErrorYaraScanTargetParams{
			ScanID: scan.ID,
			NodeID: node.ID,
			Error:  errMsg,
		}); err != nil {
			return true, fmt.Errorf("mark yara scan target error: %w", err)
		}
		return true, c.store.RefreshYaraScanStats(ctx, scan.ID)
	}

	for _, row := range yaraRows(rows) {
		count := int32(parseInt(row["count"]))
		matches := strings.TrimSpace(row["matches"])
		if count <= 0 || matches == "" {
			continue
		}
		if err := c.store.InsertYaraScanMatch(ctx, repo.InsertYaraScanMatchParams{
			ScanID:  scan.ID,
			NodeID:  node.ID,
			Path:    strings.TrimSpace(row["path"]),
			Matches: matches,
			Count:   count,
		}); err != nil {
			return true, fmt.Errorf("insert yara scan match: %w", err)
		}
	}

	if err := c.store.CompleteYaraScanTarget(ctx, repo.CompleteYaraScanTargetParams{
		ScanID: scan.ID,
		NodeID: node.ID,
	}); err != nil {
		return true, fmt.Errorf("complete yara scan target: %w", err)
	}
	return true, c.store.RefreshYaraScanStats(ctx, scan.ID)
}

func (c *Core) expireStaleYaraTargets(ctx context.Context) error {
	scanIDs, err := c.store.TimeoutStaleYaraScanTargets(ctx, postgresInterval(yaraScanTargetTimeout))
	if err != nil {
		return fmt.Errorf("timeout stale yara scan targets: %w", err)
	}
	seen := make(map[int64]struct{}, len(scanIDs))
	for _, scanID := range scanIDs {
		if _, ok := seen[scanID]; ok {
			continue
		}
		seen[scanID] = struct{}{}
		if err := c.store.RefreshYaraScanStats(ctx, scanID); err != nil {
			return fmt.Errorf("refresh timed out yara scan stats: %w", err)
		}
	}
	return nil
}

func (c *Core) YaraSignatureURLAllowlist(ctx context.Context, req models.NodeKeyRequest) ([]string, error) {
	node, err := c.GetNode(ctx, req)
	if err != nil {
		return nil, err
	}
	nodeUUID, err := uuid.Parse(node.UUID)
	if err != nil {
		return nil, fmt.Errorf("parse node uuid: %w", err)
	}
	groupRows, err := c.store.ListGroupsForNode(ctx, nodeUUID)
	if err != nil {
		return nil, fmt.Errorf("list node yara groups: %w", err)
	}
	sources, err := c.store.ListEnabledDefaultYaraSignatureSources(ctx)
	if err != nil {
		return nil, fmt.Errorf("list default yara sources: %w", err)
	}
	seen := make(map[string]struct{}, len(sources))
	urls := make([]string, 0, len(sources))
	for _, source := range sources {
		key := strings.ToLower(strings.TrimSpace(source.Url))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		urls = append(urls, strings.TrimSpace(source.Url))
	}
	if len(groupRows) > 0 {
		for _, group := range groupRows {
			groupSources, err := c.store.ListEnabledYaraSignatureSourcesByGroupID(ctx, sql.NullInt64{Int64: group.ID, Valid: true})
			if err != nil {
				return nil, fmt.Errorf("list group yara sources: %w", err)
			}
			for _, source := range groupSources {
				key := strings.ToLower(strings.TrimSpace(source.Url))
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				urls = append(urls, strings.TrimSpace(source.Url))
			}
		}
	}
	return urls, nil
}

func (c *Core) yaraSourceGroupID(ctx context.Context, raw string) (sql.NullInt64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return sql.NullInt64{}, nil
	}
	groupUUID, err := uuid.Parse(raw)
	if err != nil {
		return sql.NullInt64{}, fmt.Errorf("parse group uuid: %w", err)
	}
	group, err := c.store.GetGroupByUUID(ctx, groupUUID)
	if err != nil {
		return sql.NullInt64{}, fmt.Errorf("get group: %w", err)
	}
	return sql.NullInt64{Int64: group.ID, Valid: true}, nil
}

func (c *Core) getYaraSignatureSourceModel(ctx context.Context, sourceUUID uuid.UUID) (models.YaraSignatureSource, error) {
	rows, err := c.store.ListYaraSignatureSources(ctx, repo.ListYaraSignatureSourcesParams{LimitCount: 10000})
	if err != nil {
		return models.YaraSignatureSource{}, fmt.Errorf("list yara signature sources: %w", err)
	}
	for _, row := range rows {
		if row.Uuid == sourceUUID {
			return models.YaraSignatureSourceFromRow(row), nil
		}
	}
	return models.YaraSignatureSource{}, sql.ErrNoRows
}

func validateYaraSourceURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("%w: YARA signature URL must be https", ErrInvalidQuery)
	}
	return nil
}

func sqlString(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func normalizePage(req models.PageRequest) (int, int) {
	page := req.Page
	count := req.Count
	if count <= 0 {
		count = 10
	}
	if page < 0 {
		page = 0
	}
	return page, count
}

func yaraRows(raw interface{}) []map[string]string {
	var rows []map[string]interface{}
	switch v := raw.(type) {
	case []interface{}:
		for _, item := range v {
			if row, ok := item.(map[string]interface{}); ok {
				rows = append(rows, row)
			}
		}
	case []map[string]interface{}:
		rows = v
	case nil:
		return nil
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		_ = json.Unmarshal(encoded, &rows)
	}
	out := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		converted := make(map[string]string, len(row))
		for k, v := range row {
			converted[k] = stringValue(v)
		}
		out = append(out, converted)
	}
	return out
}
