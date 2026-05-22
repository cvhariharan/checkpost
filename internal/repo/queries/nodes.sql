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

-- name: TouchNode :exec
UPDATE nodes SET last_seen_at = now(), updated_at = now() WHERE node_key = $1;

-- name: ListNodes :many
WITH filtered AS (
    SELECT * FROM nodes
),
total AS (
    SELECT count(*) AS total_count FROM filtered
)
SELECT filtered.*, total.total_count
FROM filtered, total
ORDER BY filtered.created_at DESC
LIMIT $1 OFFSET $2;
