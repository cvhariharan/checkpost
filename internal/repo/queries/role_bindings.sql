-- name: CreateRoleBinding :one
INSERT INTO role_bindings (
    user_id,
    user_group_id,
    role,
    scope_group_id
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: GetRoleBindingByUUID :one
SELECT * FROM role_bindings WHERE uuid = $1;

-- name: FindRoleBinding :one
SELECT * FROM role_bindings
WHERE user_id IS NOT DISTINCT FROM @user_id
  AND user_group_id IS NOT DISTINCT FROM @user_group_id
  AND role = @role
  AND scope_group_id IS NOT DISTINCT FROM @scope_group_id
LIMIT 1;

-- name: DeleteRoleBindingByUUID :execrows
DELETE FROM role_bindings WHERE uuid = $1;

-- name: ListRoleBindingsForUser :many
SELECT
    role_bindings.id,
    role_bindings.uuid,
    role_bindings.role,
    role_bindings.created_at,
    groups.uuid AS scope_group_uuid,
    groups.name AS scope_group_name
FROM role_bindings
LEFT JOIN groups ON groups.id = role_bindings.scope_group_id
WHERE role_bindings.user_id = $1
ORDER BY role_bindings.created_at DESC;

-- name: ListRoleBindingsForUserGroup :many
SELECT
    role_bindings.id,
    role_bindings.uuid,
    role_bindings.role,
    role_bindings.created_at,
    groups.uuid AS scope_group_uuid,
    groups.name AS scope_group_name
FROM role_bindings
LEFT JOIN groups ON groups.id = role_bindings.scope_group_id
WHERE role_bindings.user_group_id = $1
ORDER BY role_bindings.created_at DESC;

-- name: ListAllRoleBindings :many
SELECT
    role_bindings.uuid,
    role_bindings.role,
    users.uuid AS user_uuid,
    user_groups.uuid AS user_group_uuid,
    groups.uuid AS scope_group_uuid
FROM role_bindings
LEFT JOIN users ON users.id = role_bindings.user_id
LEFT JOIN user_groups ON user_groups.id = role_bindings.user_group_id
LEFT JOIN groups ON groups.id = role_bindings.scope_group_id
ORDER BY role_bindings.id;

-- name: ListGlobalRolesForUser :many
SELECT DISTINCT role FROM role_bindings
WHERE user_id = $1 AND scope_group_id IS NULL;

-- name: ListGlobalRolesForUserGroups :many
SELECT DISTINCT role_bindings.role
FROM role_bindings
JOIN user_group_members ON user_group_members.user_group_id = role_bindings.user_group_id
WHERE user_group_members.user_id = $1 AND role_bindings.scope_group_id IS NULL;
