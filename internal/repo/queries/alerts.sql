-- name: CreateAlertTarget :one
INSERT INTO alert_targets (name, type, config, enabled)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetAlertTargetByUUID :one
SELECT * FROM alert_targets WHERE uuid = $1;

-- name: GetAlertTargetByName :one
SELECT * FROM alert_targets WHERE name = $1;

-- name: UpdateAlertTargetByUUID :one
UPDATE alert_targets
SET name = $2, config = $3, enabled = $4, updated_at = now()
WHERE uuid = $1
RETURNING *;

-- name: DeleteAlertTargetByUUID :execrows
DELETE FROM alert_targets WHERE uuid = $1;

-- name: ListAlertTargets :many
SELECT *, count(*) OVER () AS total_count
FROM alert_targets
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CreateAlertRule :one
INSERT INTO alert_rules (
    name, description, source, params, severity, enabled,
    evaluation_interval_seconds, for_seconds, repeat_interval_seconds
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetAlertRuleByUUID :one
SELECT * FROM alert_rules WHERE uuid = $1;

-- name: GetAlertRuleByName :one
SELECT * FROM alert_rules WHERE name = $1;

-- name: UpdateAlertRuleByUUID :one
UPDATE alert_rules
SET name = $2, description = $3, source = $4, params = $5, severity = $6,
    enabled = $7, evaluation_interval_seconds = $8, for_seconds = $9,
    repeat_interval_seconds = $10, updated_at = now()
WHERE uuid = $1
RETURNING *;

-- name: DeleteAlertRuleByUUID :execrows
DELETE FROM alert_rules WHERE uuid = $1;

-- name: ListAlertRules :many
SELECT *, count(*) OVER () AS total_count
FROM alert_rules
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ClaimDueAlertRules :many
UPDATE alert_rules
SET last_evaluated_at = now()
WHERE enabled = true
  AND (last_evaluated_at IS NULL
       OR now() - last_evaluated_at >= make_interval(secs => evaluation_interval_seconds))
RETURNING *;

-- name: CreateAlertRuleTarget :exec
INSERT INTO alert_rule_targets (rule_id, target_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: DeleteAlertRuleTargetsForRule :exec
DELETE FROM alert_rule_targets WHERE rule_id = $1;

-- name: ListTargetsForRule :many
SELECT t.* FROM alert_targets t
JOIN alert_rule_targets rt ON rt.target_id = t.id
WHERE rt.rule_id = $1 AND t.enabled = true;

-- name: ListTargetUUIDsForRule :many
SELECT t.uuid FROM alert_targets t
JOIN alert_rule_targets rt ON rt.target_id = t.id
WHERE rt.rule_id = $1;

-- name: ListAlertStateByRule :many
SELECT * FROM alert_state WHERE rule_id = $1;

-- name: UpsertAlertState :exec
INSERT INTO alert_state (rule_id, alert_key, status, labels, annotations, first_seen_at, last_seen_at, last_notified_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (rule_id, alert_key) DO UPDATE SET
    status = EXCLUDED.status,
    labels = EXCLUDED.labels,
    annotations = EXCLUDED.annotations,
    last_seen_at = EXCLUDED.last_seen_at,
    last_notified_at = EXCLUDED.last_notified_at;

-- name: DeleteAlertState :exec
DELETE FROM alert_state WHERE rule_id = $1 AND alert_key = $2;

-- name: ListFailingPolicyNodes :many
SELECT
    p.uuid AS policy_uuid,
    p.name AS policy_name,
    p.resolution AS resolution,
    n.id AS node_id,
    n.uuid AS node_uuid,
    COALESCE(NULLIF(n.display_name, ''), n.hostname) AS hostname,
    n.platform AS platform
FROM policy_membership pm
JOIN policies p ON p.id = pm.policy_id
JOIN nodes n ON n.id = pm.node_id
WHERE pm.passes = false
  AND (
    (coalesce(cardinality(@policies::uuid[]), 0) > 0 AND p.uuid = ANY(@policies::uuid[]))
    OR (coalesce(cardinality(@policies::uuid[]), 0) = 0 AND p.enabled = true)
  )
  -- only nodes still within the policy's effective scope, so alerts resolve
  -- once a policy's platform/group targeting no longer covers the node
  AND (
    p.platform IN ('all', 'any')
    OR p.platform = n.platform
    OR (p.platform = 'linux' AND n.platform NOT IN ('', 'darwin', 'windows'))
    OR (p.platform = 'posix' AND n.platform NOT IN ('', 'windows'))
  )
  AND (
    NOT EXISTS (
        SELECT 1 FROM policy_groups pg WHERE pg.policy_id = p.id
    )
    OR EXISTS (
        SELECT 1 FROM policy_groups pg
        JOIN group_membership pgm ON pgm.group_id = pg.group_id
        WHERE pg.policy_id = p.id AND pgm.node_id = n.id
    )
  )
  AND (coalesce(cardinality(@platforms::text[]), 0) = 0 OR n.platform = ANY(@platforms::text[]))
  AND (
    coalesce(cardinality(@groups::uuid[]), 0) = 0
    OR EXISTS (
        SELECT 1 FROM group_membership gm
        JOIN groups g ON g.id = gm.group_id
        WHERE gm.node_id = n.id AND g.uuid = ANY(@groups::uuid[])
    )
  )
  AND (sqlc.arg(include_stale)::bool OR pm.checked_at >= now() - sqlc.arg(stale_after)::text::interval)
ORDER BY n.id;

-- name: ListOfflineNodes :many
SELECT
    n.id AS node_id,
    n.uuid AS node_uuid,
    COALESCE(NULLIF(n.display_name, ''), n.hostname) AS hostname,
    n.platform AS platform,
    n.last_seen_at AS last_seen_at
FROM nodes n
WHERE (n.last_seen_at IS NULL OR n.last_seen_at < now() - sqlc.arg(threshold)::text::interval)
  AND (coalesce(cardinality(@platforms::text[]), 0) = 0 OR n.platform = ANY(@platforms::text[]))
  AND (
    coalesce(cardinality(@groups::uuid[]), 0) = 0
    OR EXISTS (
        SELECT 1 FROM group_membership gm
        JOIN groups g ON g.id = gm.group_id
        WHERE gm.node_id = n.id AND g.uuid = ANY(@groups::uuid[])
    )
  )
ORDER BY n.id;

-- name: GetNodeOwnerLabels :one
SELECT o.email, o.external_id, o.display_name
FROM node_inventory ni
JOIN device_owners o ON o.id = ni.owner_id
WHERE ni.node_id = $1;

-- name: ListUserGroupMemberEmails :many
SELECT u.email FROM user_groups g
JOIN user_group_members m ON m.user_group_id = g.id
JOIN users u ON u.id = m.user_id
WHERE g.name = $1 AND u.disabled = false AND length(trim(u.email)) > 0;
