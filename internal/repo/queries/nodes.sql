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

-- name: GetNodeByKey :one
SELECT * FROM nodes WHERE node_key = $1;

-- name: GetNodeByUUID :one
SELECT * FROM nodes WHERE uuid = $1;

-- name: GetNodeByID :one
SELECT * FROM nodes WHERE id = $1;

-- name: ListNodesByIDs :many
SELECT id, uuid, hostname FROM nodes WHERE id = ANY(@ids::bigint[]);

-- name: MatchNodesByIdentityPattern :many
-- @pattern is expected to be pre-escaped against LIKE wildcards by the caller.
SELECT id, uuid::text AS uuid, hostname
FROM nodes
WHERE hostname ILIKE '%' || @pattern::text || '%' ESCAPE '\'
   OR uuid::text ILIKE '%' || @pattern::text || '%' ESCAPE '\'
   OR host_identifier ILIKE '%' || @pattern::text || '%' ESCAPE '\'
ORDER BY hostname ASC
LIMIT @max_count::int;

-- name: TouchNode :exec
UPDATE nodes SET last_seen_at = now(), updated_at = now() WHERE node_key = $1;

-- name: ListNodes :many
WITH filtered AS (
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
        nodes.updated_at
    FROM nodes
    LEFT JOIN node_inventory ON node_inventory.node_id = nodes.id
    LEFT JOIN device_owners ON device_owners.id = node_inventory.owner_id
    WHERE (
        @query::text = ''
        OR nodes.hostname ILIKE '%' || @query::text || '%'
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
),
total AS (
    SELECT count(*) AS total_count FROM filtered
)
SELECT filtered.*, total.total_count
FROM filtered, total
ORDER BY filtered.created_at DESC
LIMIT @limit OFFSET @offset;
