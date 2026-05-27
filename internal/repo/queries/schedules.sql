-- name: CreateSchedule :one
INSERT INTO schedules (
    name,
    sql,
    description,
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
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
)
RETURNING *;

-- name: GetScheduleByUUID :one
SELECT * FROM schedules WHERE uuid = $1;

-- name: GetScheduleByName :one
SELECT * FROM schedules WHERE name = $1;

-- name: ListSchedulesByNames :many
SELECT * FROM schedules WHERE name = ANY(@names::text[]);

-- name: ListSchedules :many
WITH filtered AS (
    SELECT * FROM schedules
),
total AS (
    SELECT count(*) AS total_count FROM filtered
)
SELECT filtered.*, total.total_count
FROM filtered, total
ORDER BY filtered.created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListEnabledSchedules :many
SELECT * FROM schedules
WHERE enabled = true
ORDER BY name
LIMIT $1;

-- name: ListEnabledSchedulesForNode :many
SELECT schedules.* FROM schedules
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

-- name: ListScheduleRetentions :many
SELECT uuid, retention_days FROM schedules;

-- name: BumpScheduleVersion :one
UPDATE schedules
SET sql_version = sql_version + 1, updated_at = now()
WHERE id = $1
RETURNING id, uuid, name, sql_version;

-- name: UpdateScheduleByUUID :one
UPDATE schedules SET
    name = $2,
    sql = $3,
    description = $4,
    interval_seconds = $5,
    platform = $6,
    version = $7,
    shard = $8,
    denylist = $9,
    removed = $10,
    snapshot = $11,
    enabled = $12,
    retention_days = $13,
    updated_at = now()
WHERE uuid = $1 AND is_system = false
RETURNING *;

-- name: DeleteScheduleByUUID :execrows
DELETE FROM schedules WHERE uuid = $1 AND is_system = false;
