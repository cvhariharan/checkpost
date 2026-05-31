-- name: CreateUserGroup :one
INSERT INTO user_groups (
    name,
    description,
    oidc_claim_value
) VALUES (
    $1, $2, $3
)
RETURNING *;

-- name: GetUserGroupByUUID :one
SELECT * FROM user_groups WHERE uuid = $1;

-- name: GetUserGroupByID :one
SELECT * FROM user_groups WHERE id = $1;

-- name: ListUserGroups :many
WITH filtered AS (
    SELECT * FROM user_groups
),
total AS (
    SELECT count(*) AS total_count FROM filtered
)
SELECT
    filtered.*,
    total.total_count,
    count(DISTINCT user_group_members.user_id)::bigint AS member_count
FROM filtered
CROSS JOIN total
LEFT JOIN user_group_members ON user_group_members.user_group_id = filtered.id
GROUP BY
    filtered.id,
    filtered.uuid,
    filtered.name,
    filtered.description,
    filtered.oidc_claim_value,
    filtered.created_at,
    filtered.updated_at,
    total.total_count
ORDER BY filtered.created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: UpdateUserGroupByUUID :one
UPDATE user_groups SET
    name = $2,
    description = $3,
    oidc_claim_value = $4,
    updated_at = now()
WHERE uuid = $1
RETURNING *;

-- name: DeleteUserGroupByUUID :execrows
DELETE FROM user_groups WHERE uuid = $1;

-- name: ListUserGroupMembers :many
SELECT
    users.id,
    users.uuid,
    users.username,
    users.name,
    users.email,
    users.login_type,
    users.disabled,
    user_group_members.source,
    user_group_members.created_at AS member_since
FROM user_group_members
JOIN users ON users.id = user_group_members.user_id
WHERE user_group_members.user_group_id = $1
ORDER BY users.username;

-- name: ListUserGroupsForUser :many
SELECT user_groups.*
FROM user_groups
JOIN user_group_members ON user_group_members.user_group_id = user_groups.id
WHERE user_group_members.user_id = $1
ORDER BY user_groups.name;

-- name: AddUserGroupMemberManual :exec
INSERT INTO user_group_members (
    user_group_id,
    user_id,
    source
) VALUES (
    $1, $2, 'manual'
)
ON CONFLICT (user_group_id, user_id) DO UPDATE SET source = 'manual';

-- name: UpsertUserGroupMemberOIDC :exec
INSERT INTO user_group_members (
    user_group_id,
    user_id,
    source
) VALUES (
    $1, $2, 'oidc'
)
ON CONFLICT (user_group_id, user_id) DO NOTHING;

-- name: RemoveUserGroupMember :exec
DELETE FROM user_group_members
WHERE user_group_id = $1 AND user_id = $2;

-- name: ListUserGroupsByClaimValues :many
SELECT * FROM user_groups
WHERE length(trim(oidc_claim_value)) > 0
  AND lower(oidc_claim_value) = ANY(@claim_values::text[]);

-- name: DeleteStaleOIDCMembers :exec
DELETE FROM user_group_members
WHERE user_id = @user_id
  AND source = 'oidc'
  AND NOT (user_group_id = ANY(@keep_group_ids::bigint[]));

-- name: DeleteAllOIDCMembersForUser :exec
DELETE FROM user_group_members
WHERE user_id = $1 AND source = 'oidc';
