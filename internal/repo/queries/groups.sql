-- name: CreateGroup :one
INSERT INTO groups (
    name,
    description
) VALUES (
    $1, $2
)
RETURNING *;

-- name: GetGroupByUUID :one
SELECT * FROM groups WHERE uuid = $1;

-- name: GetGroupWithCountsByUUID :one
SELECT
    groups.id,
    groups.uuid,
    groups.name,
    groups.description,
    groups.created_at,
    groups.updated_at,
    count(DISTINCT group_membership.node_id)::bigint AS machine_count,
    count(DISTINCT policy_groups.policy_id)::bigint AS policy_count
FROM groups
LEFT JOIN group_membership ON group_membership.group_id = groups.id
LEFT JOIN policy_groups ON policy_groups.group_id = groups.id
WHERE groups.uuid = @group_uuid
GROUP BY
    groups.id,
    groups.uuid,
    groups.name,
    groups.description,
    groups.created_at,
    groups.updated_at;

-- name: ListGroupsWithCounts :many
WITH filtered AS (
    SELECT *
    FROM groups
),
total AS (
    SELECT count(*) AS total_count FROM filtered
)
SELECT
    filtered.id,
    filtered.uuid,
    filtered.name,
    filtered.description,
    filtered.created_at,
    filtered.updated_at,
    count(DISTINCT group_membership.node_id)::bigint AS machine_count,
    count(DISTINCT policy_groups.policy_id)::bigint AS policy_count,
    total.total_count
FROM filtered
CROSS JOIN total
LEFT JOIN group_membership ON group_membership.group_id = filtered.id
LEFT JOIN policy_groups ON policy_groups.group_id = filtered.id
GROUP BY
    filtered.id,
    filtered.uuid,
    filtered.name,
    filtered.description,
    filtered.created_at,
    filtered.updated_at,
    total.total_count
ORDER BY filtered.created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: UpdateGroupByUUID :one
UPDATE groups SET
    name = $2,
    description = $3,
    updated_at = now()
WHERE uuid = $1
RETURNING *;

-- name: DeleteGroupByUUID :execrows
DELETE FROM groups WHERE uuid = $1;

-- name: ListGroupsForNode :many
SELECT
    groups.id,
    groups.uuid,
    groups.name,
    groups.description,
    groups.created_at,
    groups.updated_at
FROM groups
JOIN group_membership ON group_membership.group_id = groups.id
JOIN nodes ON nodes.id = group_membership.node_id
WHERE nodes.uuid = @node_uuid
ORDER BY groups.name;

-- name: DeleteGroupMembershipsForNode :exec
DELETE FROM group_membership
USING nodes
WHERE group_membership.node_id = nodes.id
  AND nodes.uuid = @node_uuid;

-- name: DeleteGroupMembershipForNode :exec
DELETE FROM group_membership
USING groups, nodes
WHERE group_membership.group_id = groups.id
  AND group_membership.node_id = nodes.id
  AND groups.uuid = @group_uuid
  AND nodes.uuid = @node_uuid;

-- name: CreateGroupMembership :exec
INSERT INTO group_membership (
    group_id,
    node_id
) VALUES (
    $1, $2
)
ON CONFLICT (group_id, node_id) DO NOTHING;

-- name: ListNodesByGroup :many
WITH target_group AS (
    SELECT *
    FROM groups
    WHERE groups.uuid = @group_uuid
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
        nodes.updated_at
    FROM target_group
    JOIN group_membership ON group_membership.group_id = target_group.id
    JOIN nodes ON nodes.id = group_membership.node_id
),
total AS (
    SELECT count(*) AS total_count FROM filtered
)
SELECT filtered.*, total.total_count
FROM filtered, total
ORDER BY filtered.hostname, filtered.created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: ListGroupsForPolicy :many
SELECT
    groups.id,
    groups.uuid,
    groups.name,
    groups.description,
    groups.created_at,
    groups.updated_at
FROM groups
JOIN policy_groups ON policy_groups.group_id = groups.id
JOIN policies ON policies.id = policy_groups.policy_id
WHERE policies.uuid = @policy_uuid
ORDER BY groups.name;

-- name: DeletePolicyGroupsForPolicy :exec
DELETE FROM policy_groups
USING policies
WHERE policy_groups.policy_id = policies.id
  AND policies.uuid = @policy_uuid;

-- name: CreatePolicyGroup :exec
INSERT INTO policy_groups (
    policy_id,
    group_id
) VALUES (
    $1, $2
)
ON CONFLICT (policy_id, group_id) DO NOTHING;

-- name: ListGroupsForSchedule :many
SELECT
    groups.id,
    groups.uuid,
    groups.name,
    groups.description,
    groups.created_at,
    groups.updated_at
FROM groups
JOIN schedule_groups ON schedule_groups.group_id = groups.id
JOIN schedules ON schedules.id = schedule_groups.schedule_id
WHERE schedules.uuid = @schedule_uuid
ORDER BY groups.name;

-- name: DeleteScheduleGroupsForSchedule :exec
DELETE FROM schedule_groups
USING schedules
WHERE schedule_groups.schedule_id = schedules.id
  AND schedules.uuid = @schedule_uuid;

-- name: CreateScheduleGroup :exec
INSERT INTO schedule_groups (
    schedule_id,
    group_id
) VALUES (
    $1, $2
)
ON CONFLICT (schedule_id, group_id) DO NOTHING;
