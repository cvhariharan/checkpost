-- name: CreateNode :one
INSERT INTO nodes (
    host_identifier,
    hostname,
    platform,
    os_name,
    os_version,
    osquery_version,
    hardware_serial
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
ON CONFLICT (host_identifier) DO UPDATE SET
    hostname = EXCLUDED.hostname,
    platform = EXCLUDED.platform,
    os_name = EXCLUDED.os_name,
    os_version = EXCLUDED.os_version,
    osquery_version = EXCLUDED.osquery_version,
    hardware_serial = EXCLUDED.hardware_serial,
    last_seen_at = now(),
    updated_at = now()
RETURNING *;

-- name: DeleteNodeByUUID :execrows
DELETE FROM nodes WHERE uuid = $1;

-- name: GetNodeByKey :one
SELECT * FROM nodes WHERE node_key = $1;

-- name: GetNodeByUUID :one
SELECT * FROM nodes WHERE uuid = $1;

-- name: GetNodeByID :one
SELECT * FROM nodes WHERE id = $1;

-- name: UpdateNodeDisplayNameByUUID :one
UPDATE nodes SET
    display_name = @display_name,
    updated_at = now()
WHERE uuid = @uuid
RETURNING *;

-- name: ListNodesByIDs :many
SELECT id, uuid, COALESCE(NULLIF(display_name, ''), hostname) AS hostname FROM nodes WHERE id = ANY(@ids::bigint[]);

-- name: MatchNodesByIdentityPattern :many
-- @pattern is expected to be pre-escaped against LIKE wildcards by the caller.
SELECT id, uuid::text AS uuid, hostname
FROM nodes
WHERE hostname ILIKE '%' || @pattern::text || '%' ESCAPE '\'
   OR display_name ILIKE '%' || @pattern::text || '%' ESCAPE '\'
   OR uuid::text ILIKE '%' || @pattern::text || '%' ESCAPE '\'
   OR host_identifier ILIKE '%' || @pattern::text || '%' ESCAPE '\'
ORDER BY hostname ASC
LIMIT @max_count::int;

-- name: TouchNode :exec
UPDATE nodes SET last_seen_at = now(), updated_at = now() WHERE node_key = $1;

-- name: UpdateNodeSystemInfo :exec
UPDATE nodes SET
    platform        = COALESCE(NULLIF(@platform::text, ''), platform),
    os_name         = COALESCE(NULLIF(@os_name::text, ''), os_name),
    os_version      = COALESCE(NULLIF(@os_version::text, ''), os_version),
    osquery_version = COALESCE(NULLIF(@osquery_version::text, ''), osquery_version),
    hardware_serial = COALESCE(NULLIF(@hardware_serial::text, ''), hardware_serial),
    updated_at      = now()
WHERE id = @node_id;

-- name: ListNodes :many
WITH filtered AS (
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
        nodes.updated_at
    FROM nodes
    LEFT JOIN node_inventory ON node_inventory.node_id = nodes.id
    LEFT JOIN device_owners ON device_owners.id = node_inventory.owner_id
    WHERE (
        @query::text = ''
        OR nodes.hostname ILIKE '%' || @query::text || '%'
        OR nodes.display_name ILIKE '%' || @query::text || '%'
        OR nodes.host_identifier ILIKE '%' || @query::text || '%'
        OR node_inventory.internal_tracking_id ILIKE '%' || @query::text || '%'
        OR device_owners.display_name ILIKE '%' || @query::text || '%'
        OR device_owners.email ILIKE '%' || @query::text || '%'
    )
      AND (
        @platform::text = ''
        OR nodes.platform = @platform::text
    )
      AND (
        @owner_uuid::text = ''
        OR device_owners.uuid::text = @owner_uuid::text
    )
      AND (
        @assigned::text = ''
        OR (@assigned::text = 'assigned' AND node_inventory.owner_id IS NOT NULL)
        OR (@assigned::text = 'unassigned' AND node_inventory.owner_id IS NULL)
    )
      AND (
        @owner_email::text = ''
        OR lower(trim(device_owners.email)) = lower(trim(@owner_email::text))
    )
),
total AS (
    SELECT count(*) AS total_count FROM filtered
),
page AS (
    SELECT filtered.*
    FROM filtered
    ORDER BY filtered.created_at DESC
    LIMIT @limit_count OFFSET @offset_count
)
SELECT
    page.*,
    total.total_count,
    coalesce(score.weighted_passing, 0)::bigint AS weighted_passing,
    coalesce(score.weighted_total, 0)::bigint AS weighted_total
FROM page, total
LEFT JOIN LATERAL (
    SELECT
        coalesce(sum(
            CASE policies.severity
                WHEN 'critical' THEN 8 WHEN 'high' THEN 5 WHEN 'medium' THEN 3 WHEN 'low' THEN 2 WHEN 'info' THEN 1 ELSE 3
            END
        ) FILTER (WHERE policy_membership.passes = true AND policy_membership.checked_at >= @stale_cutoff::timestamptz), 0)::bigint AS weighted_passing,
        coalesce(sum(
            CASE policies.severity
                WHEN 'critical' THEN 8 WHEN 'high' THEN 5 WHEN 'medium' THEN 3 WHEN 'low' THEN 2 WHEN 'info' THEN 1 ELSE 3
            END
        ), 0)::bigint AS weighted_total
    FROM policies
    LEFT JOIN policy_membership
        ON policy_membership.policy_id = policies.id
       AND policy_membership.node_id = page.id
    WHERE policies.enabled = true
        AND (
            policies.platform IN ('all', 'any')
            OR policies.platform = page.platform
            OR (policies.platform = 'linux' AND page.platform NOT IN ('', 'darwin', 'windows'))
            OR (policies.platform = 'posix' AND page.platform NOT IN ('', 'windows'))
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
                  AND group_membership.node_id = page.id
            )
        )
) score ON true
ORDER BY page.created_at DESC;
