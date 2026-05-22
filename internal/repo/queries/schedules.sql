-- name: CreateSchedule :one
INSERT INTO schedules (
    query_id,
    name,
    interval_seconds,
    platform,
    version,
    shard,
    denylist,
    removed,
    snapshot,
    enabled,
    is_system
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING *;

-- name: GetScheduleByUUID :one
SELECT * FROM schedules WHERE uuid = $1;

-- name: GetScheduleWithQueryByUUID :one
SELECT
    schedules.id,
    schedules.uuid,
    schedules.query_id,
    schedules.name,
    schedules.interval_seconds,
    schedules.platform,
    schedules.version,
    schedules.shard,
    schedules.denylist,
    schedules.removed,
    schedules.snapshot,
    schedules.enabled,
    schedules.is_system,
    schedules.created_at,
    schedules.updated_at,
    queries.id AS query_id_value,
    queries.uuid AS query_uuid,
    queries.name AS query_name,
    queries.sql AS query_sql,
    queries.description AS query_description,
    queries.is_system AS query_is_system,
    queries.created_at AS query_created_at,
    queries.updated_at AS query_updated_at
FROM schedules
JOIN queries ON queries.id = schedules.query_id
WHERE schedules.uuid = $1;

-- name: ListSchedulesWithQueries :many
WITH filtered AS (
    SELECT
        schedules.id,
        schedules.uuid,
        schedules.query_id,
        schedules.name,
        schedules.interval_seconds,
        schedules.platform,
        schedules.version,
        schedules.shard,
        schedules.denylist,
        schedules.removed,
        schedules.snapshot,
        schedules.enabled,
        schedules.is_system,
        schedules.created_at,
        schedules.updated_at,
        queries.id AS query_id_value,
        queries.uuid AS query_uuid,
        queries.name AS query_name,
        queries.sql AS query_sql,
        queries.description AS query_description,
        queries.is_system AS query_is_system,
        queries.created_at AS query_created_at,
        queries.updated_at AS query_updated_at
    FROM schedules
    JOIN queries ON queries.id = schedules.query_id
),
total AS (
    SELECT count(*) AS total_count FROM filtered
)
SELECT filtered.*, total.total_count
FROM filtered, total
ORDER BY filtered.created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListEnabledSchedulesWithQueries :many
SELECT
    schedules.id,
    schedules.uuid,
    schedules.query_id,
    schedules.name,
    schedules.interval_seconds,
    schedules.platform,
    schedules.version,
    schedules.shard,
    schedules.denylist,
    schedules.removed,
    schedules.snapshot,
    schedules.enabled,
    schedules.is_system,
    schedules.created_at,
    schedules.updated_at,
    queries.id AS query_id_value,
    queries.uuid AS query_uuid,
    queries.name AS query_name,
    queries.sql AS query_sql,
    queries.description AS query_description,
    queries.is_system AS query_is_system,
    queries.created_at AS query_created_at,
    queries.updated_at AS query_updated_at
FROM schedules
JOIN queries ON queries.id = schedules.query_id
WHERE schedules.enabled = true
ORDER BY schedules.name
LIMIT $1;

-- name: ListSystemScheduleNames :many
SELECT name FROM schedules WHERE is_system = true;

-- name: UpdateScheduleByUUID :one
UPDATE schedules SET
    query_id = $2,
    name = $3,
    interval_seconds = $4,
    platform = $5,
    version = $6,
    shard = $7,
    denylist = $8,
    removed = $9,
    snapshot = $10,
    enabled = $11,
    updated_at = now()
WHERE uuid = $1 AND is_system = false
RETURNING *;

-- name: DeleteScheduleByUUID :execrows
DELETE FROM schedules WHERE uuid = $1 AND is_system = false;
