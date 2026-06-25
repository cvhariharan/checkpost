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

-- name: GetPolicyByName :one
SELECT * FROM policies WHERE name = $1;

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
    SELECT id, uuid, name, query, description, resolution, platform, enabled, is_system, created_at, updated_at
    FROM policies
    WHERE (
        @query::text = ''
        OR name ILIKE '%' || @query::text || '%'
        OR description ILIKE '%' || @query::text || '%'
        OR query ILIKE '%' || @query::text || '%'
    )
),
total AS (
    SELECT count(*) AS total_count FROM filtered
),
page AS (
    SELECT *
    FROM filtered
    ORDER BY created_at DESC, id DESC
    LIMIT @limit_count OFFSET @offset_count
),
effective_targets AS (
    SELECT
        page.id AS policy_id,
        nodes.id AS node_id
    FROM page
    JOIN nodes ON (
        page.platform IN ('all', 'any')
        OR page.platform = nodes.platform
        OR (page.platform = 'linux' AND nodes.platform NOT IN ('', 'darwin', 'windows'))
        OR (page.platform = 'posix' AND nodes.platform NOT IN ('', 'windows'))
    )
    WHERE (
        NOT EXISTS (
            SELECT 1
            FROM policy_groups
            WHERE policy_groups.policy_id = page.id
        )
        OR EXISTS (
            SELECT 1
            FROM policy_groups
            JOIN group_membership ON group_membership.group_id = policy_groups.group_id
            WHERE policy_groups.policy_id = page.id
              AND group_membership.node_id = nodes.id
        )
    )
),
counts AS (
    SELECT
        page.id AS policy_id,
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
        max(policy_membership.updated_at) AS last_count_updated_at
    FROM page
    LEFT JOIN effective_targets ON effective_targets.policy_id = page.id
    LEFT JOIN policy_membership
        ON policy_membership.policy_id = page.id
       AND policy_membership.node_id = effective_targets.node_id
    GROUP BY page.id
)
SELECT
    page.id,
    page.uuid,
    page.name,
    page.query,
    page.description,
    page.resolution,
    page.platform,
    page.enabled,
    page.is_system,
    page.created_at,
    page.updated_at,
    COALESCE(counts.passing_count, 0)::bigint AS passing_count,
    COALESCE(counts.failing_count, 0)::bigint AS failing_count,
    COALESCE(counts.unknown_count, 0)::bigint AS unknown_count,
    counts.last_count_updated_at,
    total.total_count,
    groups.id AS group_id,
    groups.uuid AS group_uuid,
    groups.name AS group_name,
    groups.description AS group_description,
    groups.created_at AS group_created_at,
    groups.updated_at AS group_updated_at
FROM page
CROSS JOIN total
LEFT JOIN counts ON counts.policy_id = page.id
LEFT JOIN policy_groups ON policy_groups.policy_id = page.id
LEFT JOIN groups ON groups.id = policy_groups.group_id
ORDER BY page.created_at DESC, page.id DESC, groups.name;

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
        nodes.display_name,
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
