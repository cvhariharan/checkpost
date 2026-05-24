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

-- name: GetScheduleByName :one
SELECT * FROM schedules WHERE name = $1;

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
    schedules.sql_version,
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
        schedules.sql_version,
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
    schedules.sql_version,
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

-- name: ListEnabledSchedulesForNode :many
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
    schedules.sql_version,
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
  AND (
    NOT EXISTS (
        SELECT 1
        FROM schedule_groups
        WHERE schedule_groups.schedule_id = schedules.id
    )
    OR EXISTS (
        SELECT 1
        FROM schedule_groups
        JOIN group_membership ON group_membership.group_id = schedule_groups.group_id
        WHERE schedule_groups.schedule_id = schedules.id
          AND group_membership.node_id = @node_id
    )
  )
ORDER BY schedules.name
LIMIT @limit_count;

-- name: ListSchedulesForQuery :many
SELECT id, uuid, name, sql_version FROM schedules WHERE query_id = $1;

-- name: ListScheduleRetentions :many
SELECT uuid, retention_days FROM schedules;

-- name: BumpScheduleVersion :one
UPDATE schedules
SET sql_version = sql_version + 1, updated_at = now()
WHERE id = $1
RETURNING id, uuid, name, sql_version;

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
    retention_days = $12,
    updated_at = now()
WHERE uuid = $1 AND is_system = false
RETURNING *;

-- name: DeleteScheduleByUUID :execrows
DELETE FROM schedules WHERE uuid = $1 AND is_system = false;
