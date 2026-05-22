package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/cvhariharan/watcher/internal/models"
	"github.com/cvhariharan/watcher/internal/repo"
	"github.com/google/uuid"
)

const (
	policyQueryPrefix = "watcher_policy_"
	policyQueryFormat = policyQueryPrefix + "%s"
	noPoliciesQueryID = "watcher_no_policies"
)

func (c *Core) CreatePolicy(ctx context.Context, req models.CreatePolicy) (models.Policy, error) {
	if err := validatePolicyQuery(req.Query); err != nil {
		return models.Policy{}, err
	}

	groupUUIDs, err := parseUUIDList(req.GroupIDs, "group")
	if err != nil {
		return models.Policy{}, err
	}

	policy, err := c.store.CreatePolicyTx(ctx, repo.CreatePolicyTxParams{
		Policy: repo.CreatePolicyParams{
			Name:        strings.TrimSpace(req.Name),
			Query:       strings.TrimSpace(req.Query),
			Description: req.Description,
			Resolution:  req.Resolution,
			Platform:    defaultString(req.Platform, "all"),
			Enabled:     req.Enabled,
			IsSystem:    req.IsSystem,
		},
		GroupUUIDs: groupUUIDs,
	})
	if err != nil {
		return models.Policy{}, fmt.Errorf("create policy: %w", err)
	}

	out := toModelPolicy(policy)
	if err := c.attachGroupsToPolicy(ctx, &out); err != nil {
		return models.Policy{}, err
	}
	return out, nil
}

func (c *Core) GetPolicy(ctx context.Context, req models.ResourceID) (models.Policy, error) {
	policyID, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.Policy{}, fmt.Errorf("parse policy uuid: %w", err)
	}

	policy, err := c.store.GetPolicyByUUID(ctx, policyID)
	if err != nil {
		return models.Policy{}, fmt.Errorf("get policy: %w", err)
	}

	out := toModelPolicy(policy)
	if err := c.attachGroupsToPolicy(ctx, &out); err != nil {
		return models.Policy{}, err
	}
	return out, nil
}

func (c *Core) PaginatePolicies(ctx context.Context, req models.PageRequest) (models.Page[models.Policy], error) {
	page := req.Page
	countPerPage := req.Count
	if countPerPage <= 0 {
		countPerPage = 10
	}
	if page < 0 {
		page = 0
	}

	rows, err := c.store.ListPoliciesWithCounts(ctx, repo.ListPoliciesWithCountsParams{
		StaleAfter:  postgresInterval(c.policyStaleAfter),
		LimitCount:  int32(countPerPage),
		OffsetCount: int32(page * countPerPage),
	})
	if err != nil {
		return models.Page[models.Policy]{}, fmt.Errorf("list policies: %w", err)
	}

	out := make([]models.Policy, 0, len(rows))
	totalCount := 0
	for _, row := range rows {
		out = append(out, toModelPolicyRow(row))
		totalCount = int(row.TotalCount)
	}

	if err := c.attachGroupsToPolicies(ctx, out); err != nil {
		return models.Page[models.Policy]{}, err
	}

	return models.Page[models.Policy]{
		Items:      out,
		TotalCount: totalCount,
		PageCount:  pageCountFor(totalCount, countPerPage),
	}, nil
}

func (c *Core) UpdatePolicy(ctx context.Context, req models.UpdatePolicy) (models.Policy, error) {
	if err := validatePolicyQuery(req.Query); err != nil {
		return models.Policy{}, err
	}

	policyID, err := uuid.Parse(req.UUID)
	if err != nil {
		return models.Policy{}, fmt.Errorf("parse policy uuid: %w", err)
	}

	groupUUIDs, err := parseUUIDList(req.GroupIDs, "group")
	if err != nil {
		return models.Policy{}, err
	}

	policy, err := c.store.UpdatePolicyTx(ctx, repo.UpdatePolicyTxParams{
		Policy: repo.UpdatePolicyByUUIDParams{
			Uuid:        policyID,
			Name:        strings.TrimSpace(req.Name),
			Query:       strings.TrimSpace(req.Query),
			Description: req.Description,
			Resolution:  req.Resolution,
			Platform:    defaultString(req.Platform, "all"),
			Enabled:     req.Enabled,
		},
		GroupUUIDs: groupUUIDs,
	})
	if err != nil {
		return models.Policy{}, fmt.Errorf("update policy: %w", err)
	}

	out := toModelPolicy(policy)
	if err := c.attachGroupsToPolicy(ctx, &out); err != nil {
		return models.Policy{}, err
	}
	return out, nil
}

func (c *Core) DeletePolicy(ctx context.Context, req models.ResourceID) error {
	policyID, err := uuid.Parse(req.UUID)
	if err != nil {
		return fmt.Errorf("parse policy uuid: %w", err)
	}

	rows, err := c.store.DeletePolicyByUUID(ctx, policyID)
	if err != nil {
		return fmt.Errorf("delete policy: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (c *Core) ListPoliciesForNode(ctx context.Context, req models.NodeIdentity) ([]models.PolicyPosture, error) {
	nodeID, err := uuid.Parse(req.ID)
	if err != nil {
		return nil, fmt.Errorf("parse node uuid: %w", err)
	}

	rows, err := c.store.ListPoliciesForNode(ctx, repo.ListPoliciesForNodeParams{
		NodeUuid:   nodeID,
		StaleAfter: postgresInterval(c.policyStaleAfter),
	})
	if err != nil {
		return nil, fmt.Errorf("list node policies: %w", err)
	}

	out := make([]models.PolicyPosture, 0, len(rows))
	for _, row := range rows {
		out = append(out, toModelPolicyPosture(row))
	}
	if err := c.attachGroupsToPolicyPostures(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Core) PaginatePolicyMachines(ctx context.Context, req models.PolicyMachinesRequest) (models.Page[models.PolicyMachine], error) {
	policyID, err := uuid.Parse(req.PolicyUUID)
	if err != nil {
		return models.Page[models.PolicyMachine]{}, fmt.Errorf("parse policy uuid: %w", err)
	}

	page := req.Page
	countPerPage := req.Count
	if countPerPage <= 0 {
		countPerPage = 10
	}
	if page < 0 {
		page = 0
	}

	response := strings.ToLower(strings.TrimSpace(req.Response))
	switch response {
	case "", "passing", "failing", "unknown":
	default:
		return models.Page[models.PolicyMachine]{}, fmt.Errorf("invalid policy response %q", req.Response)
	}

	rows, err := c.store.ListNodesByPolicyResponse(ctx, repo.ListNodesByPolicyResponseParams{
		PolicyUuid:  policyID,
		Response:    response,
		StaleAfter:  postgresInterval(c.policyStaleAfter),
		LimitCount:  int32(countPerPage),
		OffsetCount: int32(page * countPerPage),
	})
	if err != nil {
		return models.Page[models.PolicyMachine]{}, fmt.Errorf("list policy machines: %w", err)
	}

	out := make([]models.PolicyMachine, 0, len(rows))
	totalCount := 0
	for _, row := range rows {
		out = append(out, toModelPolicyMachine(row))
		totalCount = int(row.TotalCount)
	}

	for i := range out {
		if err := c.attachGroupsToNode(ctx, &out[i].Node); err != nil {
			return models.Page[models.PolicyMachine]{}, err
		}
	}

	return models.Page[models.PolicyMachine]{
		Items:      out,
		TotalCount: totalCount,
		PageCount:  pageCountFor(totalCount, countPerPage),
	}, nil
}

func (c *Core) attachGroupsToPolicyPostures(ctx context.Context, policies []models.PolicyPosture) error {
	for i := range policies {
		if err := c.attachGroupsToPolicy(ctx, &policies[i].Policy); err != nil {
			return err
		}
	}
	return nil
}

func (c *Core) recordPolicyResults(ctx context.Context, node models.Node, results map[string]interface{}, statuses, messages map[string]string) error {
	policySeen := false
	var firstErr error
	for queryID, rows := range results {
		if !isPolicyDistributedQuery(queryID) {
			continue
		}
		policySeen = true
		if queryID == noPoliciesQueryID {
			continue
		}
		if err := c.recordPolicyResult(ctx, node, queryID, rows, statuses[queryID], messages[queryID]); err != nil {
			c.logger.Error("record policy result", "node_id", node.ID, "query_id", queryID, "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	if !policySeen {
		return firstErr
	}

	if err := c.store.UpdateNodeLastPolicyCheckAt(ctx, node.ID); err != nil {
		err = fmt.Errorf("update node policy check timestamp: %w", err)
		c.logger.Error("update node policy check timestamp", "node_id", node.ID, "error", err)
		if firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

func (c *Core) recordPolicyResult(ctx context.Context, node models.Node, queryID string, rows interface{}, status string, message string) error {
	policyUUID := strings.TrimPrefix(queryID, policyQueryPrefix)
	parsedUUID, err := uuid.Parse(policyUUID)
	if err != nil {
		c.logger.Debug("ignoring malformed policy query id", "query_id", queryID, "error", err)
		return nil
	}

	policy, err := c.store.GetPolicyByUUID(ctx, parsedUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.logger.Debug("ignoring result for deleted policy", "policy_uuid", policyUUID)
			return nil
		}
		return fmt.Errorf("get policy for result: %w", err)
	}

	passes := sql.NullBool{}
	lastError := distributedErrorMessage(status, message)
	if lastError == "" {
		if result, ok := policyResultPasses(rows); ok {
			passes = sql.NullBool{Bool: result, Valid: true}
		} else {
			lastError = "malformed policy result"
		}
	}

	if err := c.store.UpsertPolicyMembership(ctx, repo.UpsertPolicyMembershipParams{
		PolicyID:  policy.ID,
		NodeID:    node.ID,
		Passes:    passes,
		LastError: lastError,
	}); err != nil {
		return fmt.Errorf("upsert policy membership: %w", err)
	}
	return nil
}

func policyResultPasses(rows interface{}) (passes bool, ok bool) {
	switch v := rows.(type) {
	case nil:
		return false, true
	case []interface{}:
		if len(v) == 0 {
			return false, true
		}
		return inspectPolicyRow(v[0])
	case []map[string]interface{}:
		if len(v) == 0 {
			return false, true
		}
		return inspectPolicyRow(v[0])
	case []map[string]string:
		if len(v) == 0 {
			return false, true
		}
		values := make([]string, 0, len(v[0]))
		for _, val := range v[0] {
			values = append(values, val)
		}
		return classifyPolicyValues(values)
	default:
		return false, false
	}
}

func inspectPolicyRow(row interface{}) (bool, bool) {
	switch m := row.(type) {
	case map[string]interface{}:
		values := make([]string, 0, len(m))
		for _, val := range m {
			if s, ok := normalizePolicyValue(val); ok {
				values = append(values, s)
			}
		}
		return classifyPolicyValues(values)
	case map[string]string:
		values := make([]string, 0, len(m))
		for _, val := range m {
			values = append(values, val)
		}
		return classifyPolicyValues(values)
	}
	return false, false
}

func normalizePolicyValue(v interface{}) (string, bool) {
	switch s := v.(type) {
	case string:
		return s, true
	case json.Number:
		return s.String(), true
	case float64:
		if math.IsNaN(s) || math.IsInf(s, 0) {
			return "", false
		}
		if s == math.Trunc(s) && s >= math.MinInt64 && s <= math.MaxInt64 {
			return strconv.FormatInt(int64(s), 10), true
		}
		return strconv.FormatFloat(s, 'f', -1, 64), true
	case bool:
		if s {
			return "1", true
		}
		return "0", true
	}
	return "", false
}

func classifyPolicyValues(values []string) (bool, bool) {
	hasPass, hasFail := false, false
	for _, val := range values {
		switch strings.TrimSpace(val) {
		case "1":
			hasPass = true
		case "0":
			hasFail = true
		}
	}
	if hasPass {
		return true, true
	}
	if hasFail {
		return false, true
	}
	return false, false
}

func validatePolicyQuery(query string) error {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return fmt.Errorf("%w: policy query cannot be empty", ErrInvalidQuery)
	}

	withoutTrailingSemicolon := strings.TrimSpace(strings.TrimSuffix(trimmed, ";"))
	if strings.Contains(withoutTrailingSemicolon, ";") {
		return fmt.Errorf("%w: policy query must be a single statement", ErrInvalidQuery)
	}

	lower := strings.ToLower(withoutTrailingSemicolon)
	if !hasSQLKeywordPrefix(lower, "select") && !hasSQLKeywordPrefix(lower, "with") {
		return fmt.Errorf("%w: policy query must start with SELECT or WITH", ErrInvalidQuery)
	}

	return validateQuery(withoutTrailingSemicolon)
}

func hasSQLKeywordPrefix(s, keyword string) bool {
	if !strings.HasPrefix(s, keyword) {
		return false
	}
	if len(s) == len(keyword) {
		return true
	}
	switch s[len(keyword)] {
	case ' ', '\t', '\n', '\r', '(':
		return true
	}
	return false
}

func (c *Core) nodePolicyDue(node models.Node) bool {
	if node.LastPolicyCheckAt == nil {
		return true
	}
	interval := c.policyUpdateInterval + deterministicPolicyJitter(node.UUID, c.policyUpdateInterval)
	return time.Since(*node.LastPolicyCheckAt) >= interval
}

func deterministicPolicyJitter(key string, interval time.Duration) time.Duration {
	if interval <= 0 {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	fraction := float64(h.Sum32()%1000) / 1000
	return time.Duration(float64(interval) * 0.10 * fraction)
}

func isPolicyDistributedQuery(queryID string) bool {
	return queryID == noPoliciesQueryID || strings.HasPrefix(queryID, policyQueryPrefix)
}
