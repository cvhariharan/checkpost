-- name: CreateQueryRun :one
INSERT INTO query_runs (
    uuid,
    query,
    targets,
    created_by
) VALUES (
    $1, $2, sqlc.arg(targets)::text::jsonb, $3
)
RETURNING *;

-- name: GetQueryRunByUUID :one
SELECT * FROM query_runs WHERE uuid = $1;

-- name: ListQueryRuns :many
WITH filtered AS (
    SELECT query_runs.*
    FROM query_runs
),
total AS (
    SELECT count(*) AS total_count FROM filtered
)
SELECT
    filtered.*,
    total.total_count,
    (SELECT count(*) FROM machine_query_results m WHERE m.run_id = filtered.id) AS host_count,
    (SELECT count(*) FROM machine_query_results m WHERE m.run_id = filtered.id AND m.status = 'pending') AS pending_count,
    (SELECT count(*) FROM machine_query_results m WHERE m.run_id = filtered.id AND m.status = 'complete') AS complete_count,
    (SELECT count(*) FROM machine_query_results m WHERE m.run_id = filtered.id AND m.status = 'error') AS error_count
FROM filtered, total
ORDER BY filtered.created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: ListMachineQueryResultsByRunUUID :many
SELECT
    machine_query_results.id, machine_query_results.uuid, machine_query_results.node_id,
    machine_query_results.query, machine_query_results.status, machine_query_results.error,
    machine_query_results.row_count, machine_query_results.dispatched_at,
    machine_query_results.completed_at, machine_query_results.created_at,
    machine_query_results.updated_at, machine_query_results.run_id,
    nodes.uuid AS node_uuid,
    COALESCE(NULLIF(nodes.display_name, ''), nodes.hostname) AS hostname,
    nodes.platform AS platform
FROM machine_query_results
JOIN query_runs ON query_runs.id = machine_query_results.run_id
JOIN nodes ON nodes.id = machine_query_results.node_id
WHERE query_runs.uuid = @run_uuid
ORDER BY COALESCE(NULLIF(nodes.display_name, ''), nodes.hostname) ASC, nodes.uuid ASC;

-- name: DeleteQueryRunByUUID :execrows
DELETE FROM query_runs WHERE uuid = @run_uuid;

-- name: ListNodeIDsByUUIDs :many
SELECT id FROM nodes WHERE uuid = ANY(@uuids::uuid[]);

-- name: ListNodeIDsByGroupUUIDs :many
SELECT DISTINCT group_membership.node_id AS id
FROM group_membership
JOIN groups ON groups.id = group_membership.group_id
WHERE groups.uuid = ANY(@group_uuids::uuid[]);

-- name: ListNodeIDsByPlatforms :many
-- Mirrors the platform semantics in ListEnabledPoliciesForNode: 'all'/'any'
-- match everything, 'posix' matches non-windows, 'linux' matches non-darwin/windows.
SELECT id FROM nodes
WHERE 'all' = ANY(@platforms::text[])
   OR 'any' = ANY(@platforms::text[])
   OR platform = ANY(@platforms::text[])
   OR ('posix' = ANY(@platforms::text[]) AND platform NOT IN ('', 'windows'))
   OR ('linux' = ANY(@platforms::text[]) AND platform NOT IN ('', 'darwin', 'windows'));
