-- name: CreateMachineQueryResult :one
INSERT INTO machine_query_results (
    uuid,
    node_id,
    query,
    run_id
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: ListMachineQueryResultsByNodeUUID :many
WITH filtered AS (
    SELECT machine_query_results.id, machine_query_results.uuid, machine_query_results.node_id,
           machine_query_results.query, machine_query_results.status, machine_query_results.error,
           machine_query_results.row_count, machine_query_results.dispatched_at,
           machine_query_results.completed_at, machine_query_results.created_at,
           machine_query_results.updated_at, machine_query_results.run_id
    FROM machine_query_results
    JOIN nodes ON nodes.id = machine_query_results.node_id
    WHERE nodes.uuid = @node_uuid
),
total AS (
    SELECT count(*) AS total_count FROM filtered
)
SELECT filtered.*, total.total_count
FROM filtered, total
ORDER BY filtered.created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: GetMachineQueryResultByUUID :one
SELECT id, uuid, node_id, query, status, error, row_count,
       dispatched_at, completed_at, created_at, updated_at, run_id
FROM machine_query_results
WHERE uuid = $1;

-- name: ListMachineQueryUUIDsByRunUUID :many
SELECT machine_query_results.uuid
FROM machine_query_results
JOIN query_runs ON query_runs.id = machine_query_results.run_id
WHERE query_runs.uuid = @run_uuid;

-- name: ListPendingMachineQueryResults :many
SELECT *
FROM machine_query_results
WHERE node_id = $1
  AND status = 'pending'
  AND dispatched_at IS NULL
ORDER BY created_at ASC;

-- name: MarkMachineQueryResultsDispatched :exec
UPDATE machine_query_results
SET dispatched_at = COALESCE(dispatched_at, now()),
    updated_at = now()
WHERE id = ANY(sqlc.arg(ids)::bigint[]);

-- name: CompleteMachineQueryResult :one
UPDATE machine_query_results
SET status = CASE WHEN @error::text <> '' THEN 'error' ELSE 'complete' END,
    row_count = @row_count,
    error = @error,
    completed_at = now(),
    updated_at = now()
WHERE uuid = @uuid
RETURNING *;

-- name: DeleteMachineQueryResultByNodeAndUUID :execrows
DELETE FROM machine_query_results
USING nodes
WHERE machine_query_results.node_id = nodes.id
  AND nodes.uuid = @node_uuid
  AND machine_query_results.uuid = @query_uuid;
