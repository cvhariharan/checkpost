-- name: CreatePolicy :one
INSERT INTO policies (
    name,
    query,
    description,
    resolution,
    platform,
    enabled,
    is_system
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: GetPolicyByUUID :one
SELECT * FROM policies WHERE uuid = $1;

-- name: GetPolicyByID :one
SELECT * FROM policies WHERE id = $1;

-- name: ListEnabledPoliciesForNode :many
SELECT *
FROM policies
WHERE enabled = true
  AND (
    platform IN ('all', 'any')
    OR platform = @node_platform
    OR (platform = 'linux' AND @node_platform NOT IN ('', 'darwin', 'windows'))
    OR (platform = 'posix' AND @node_platform NOT IN ('', 'windows'))
  )
  AND (
    NOT EXISTS (
        SELECT 1
        FROM policy_groups
        WHERE policy_groups.policy_id = policies.id
    )
    OR EXISTS (
        SELECT 1
        FROM policy_groups
        JOIN group_membership ON group_membership.group_id = policy_groups.group_id
        WHERE policy_groups.policy_id = policies.id
          AND group_membership.node_id = @node_id
    )
  )
ORDER BY name;

-- name: ListPoliciesWithCounts :many
WITH filtered AS (
    SELECT *
    FROM policies
),
effective_targets AS (
    SELECT
        filtered.id AS policy_id,
        nodes.id AS node_id
    FROM filtered
    JOIN nodes ON (
        filtered.platform IN ('all', 'any')
        OR filtered.platform = nodes.platform
        OR (filtered.platform = 'linux' AND nodes.platform NOT IN ('', 'darwin', 'windows'))
        OR (filtered.platform = 'posix' AND nodes.platform NOT IN ('', 'windows'))
    )
    WHERE (
        NOT EXISTS (
            SELECT 1
            FROM policy_groups
            WHERE policy_groups.policy_id = filtered.id
        )
        OR EXISTS (
            SELECT 1
            FROM policy_groups
            JOIN group_membership ON group_membership.group_id = policy_groups.group_id
            WHERE policy_groups.policy_id = filtered.id
              AND group_membership.node_id = nodes.id
        )
    )
),
total AS (
    SELECT count(*) AS total_count FROM filtered
)
SELECT
    filtered.id,
    filtered.uuid,
    filtered.name,
    filtered.query,
    filtered.description,
    filtered.resolution,
    filtered.platform,
    filtered.enabled,
    filtered.is_system,
    filtered.created_at,
    filtered.updated_at,
    count(effective_targets.node_id) FILTER (
        WHERE policy_membership.passes = true
          AND policy_membership.checked_at >= now() - sqlc.arg(stale_after)::text::interval
    )::bigint AS passing_count,
    count(effective_targets.node_id) FILTER (
        WHERE policy_membership.passes = false
          AND policy_membership.checked_at >= now() - sqlc.arg(stale_after)::text::interval
    )::bigint AS failing_count,
    count(effective_targets.node_id) FILTER (
        WHERE effective_targets.node_id IS NOT NULL
          AND (
            policy_membership.policy_id IS NULL
            OR policy_membership.passes IS NULL
            OR policy_membership.checked_at < now() - sqlc.arg(stale_after)::text::interval
          )
    )::bigint AS unknown_count,
    max(policy_membership.updated_at) AS last_count_updated_at,
    total.total_count
FROM filtered
CROSS JOIN total
LEFT JOIN effective_targets ON effective_targets.policy_id = filtered.id
LEFT JOIN policy_membership
    ON policy_membership.policy_id = filtered.id
   AND policy_membership.node_id = effective_targets.node_id
GROUP BY
    filtered.id,
    filtered.uuid,
    filtered.name,
    filtered.query,
    filtered.description,
    filtered.resolution,
    filtered.platform,
    filtered.enabled,
    filtered.is_system,
    filtered.created_at,
    filtered.updated_at,
    total.total_count
ORDER BY filtered.created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: UpdatePolicyByUUID :one
UPDATE policies SET
    name = $2,
    query = $3,
    description = $4,
    resolution = $5,
    platform = $6,
    enabled = $7,
    updated_at = now()
WHERE uuid = $1 AND is_system = false
RETURNING *;

-- name: DeletePolicyByUUID :execrows
DELETE FROM policies WHERE uuid = $1 AND is_system = false;

-- name: UpsertPolicyMembership :exec
INSERT INTO policy_membership (
    policy_id,
    node_id,
    passes,
    last_error,
    checked_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, now(), now()
)
ON CONFLICT (policy_id, node_id) DO UPDATE SET
    passes = EXCLUDED.passes,
    last_error = EXCLUDED.last_error,
    checked_at = EXCLUDED.checked_at,
    updated_at = now();

-- name: UpdateNodeLastPolicyCheckAt :exec
UPDATE nodes SET last_policy_check_at = now(), updated_at = now() WHERE id = $1;

-- name: ListPoliciesForNode :many
WITH target_node AS (
    SELECT *
    FROM nodes
    WHERE nodes.uuid = @node_uuid
)
SELECT
    policies.id,
    policies.uuid,
    policies.name,
    policies.query,
    policies.description,
    policies.resolution,
    policies.platform,
    policies.enabled,
    policies.is_system,
    policies.created_at,
    policies.updated_at,
    CASE
        WHEN policy_membership.policy_id IS NULL THEN 'unknown'
        WHEN policy_membership.checked_at < now() - sqlc.arg(stale_after)::text::interval THEN 'unknown'
        WHEN policy_membership.passes = true THEN 'passing'
        WHEN policy_membership.passes = false THEN 'failing'
        ELSE 'unknown'
    END::text AS response,
    policy_membership.checked_at,
    policy_membership.last_error,
    COALESCE(policy_membership.checked_at < now() - sqlc.arg(stale_after)::text::interval, false)::boolean AS stale
FROM target_node
JOIN policies ON policies.enabled = true
    AND (
        policies.platform IN ('all', 'any')
        OR policies.platform = target_node.platform
        OR (policies.platform = 'linux' AND target_node.platform NOT IN ('', 'darwin', 'windows'))
        OR (policies.platform = 'posix' AND target_node.platform NOT IN ('', 'windows'))
    )
    AND (
        NOT EXISTS (
            SELECT 1
            FROM policy_groups
            WHERE policy_groups.policy_id = policies.id
        )
        OR EXISTS (
            SELECT 1
            FROM policy_groups
            JOIN group_membership ON group_membership.group_id = policy_groups.group_id
            WHERE policy_groups.policy_id = policies.id
              AND group_membership.node_id = target_node.id
        )
    )
LEFT JOIN policy_membership
    ON policy_membership.policy_id = policies.id
   AND policy_membership.node_id = target_node.id
ORDER BY policies.name;

-- name: ListNodesByPolicyResponse :many
WITH target_policy AS (
    SELECT *
    FROM policies
    WHERE policies.uuid = @policy_uuid
),
filtered AS (
    SELECT
        nodes.id,
        nodes.uuid,
        nodes.node_key,
        nodes.host_identifier,
        nodes.hostname,
        nodes.platform,
        nodes.os_name,
        nodes.os_version,
        nodes.osquery_version,
        nodes.hardware_serial,
        nodes.enrolled_at,
        nodes.last_seen_at,
        nodes.last_policy_check_at,
        nodes.created_at,
        nodes.updated_at,
        CASE
            WHEN policy_membership.policy_id IS NULL THEN 'unknown'
            WHEN policy_membership.checked_at < now() - sqlc.arg(stale_after)::text::interval THEN 'unknown'
            WHEN policy_membership.passes = true THEN 'passing'
            WHEN policy_membership.passes = false THEN 'failing'
            ELSE 'unknown'
        END::text AS response,
        policy_membership.checked_at,
        policy_membership.last_error,
        COALESCE(policy_membership.checked_at < now() - sqlc.arg(stale_after)::text::interval, false)::boolean AS stale
    FROM target_policy
    JOIN nodes ON (
        target_policy.platform IN ('all', 'any')
        OR target_policy.platform = nodes.platform
        OR (target_policy.platform = 'linux' AND nodes.platform NOT IN ('', 'darwin', 'windows'))
        OR (target_policy.platform = 'posix' AND nodes.platform NOT IN ('', 'windows'))
    )
    LEFT JOIN policy_membership
        ON policy_membership.policy_id = target_policy.id
       AND policy_membership.node_id = nodes.id
    WHERE (
        NOT EXISTS (
            SELECT 1
            FROM policy_groups
            WHERE policy_groups.policy_id = target_policy.id
        )
        OR EXISTS (
            SELECT 1
            FROM policy_groups
            JOIN group_membership ON group_membership.group_id = policy_groups.group_id
            WHERE policy_groups.policy_id = target_policy.id
              AND group_membership.node_id = nodes.id
        )
    )
),
response_filtered AS (
    SELECT *
    FROM filtered
    WHERE @response::text = ''
       OR response = @response::text
),
total AS (
    SELECT count(*) AS total_count FROM response_filtered
)
SELECT response_filtered.*, total.total_count
FROM response_filtered, total
ORDER BY response_filtered.hostname, response_filtered.created_at DESC
LIMIT @limit_count OFFSET @offset_count;
