-- name: CreateUser :one
INSERT INTO users (
    username,
    name,
    email,
    password_hash,
    login_type,
    disabled
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: GetUserByUUID :one
SELECT * FROM users WHERE uuid = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE lower(username) = lower(@username);

-- name: ListUsers :many
WITH filtered AS (
    SELECT * FROM users
),
total AS (
    SELECT count(*) AS total_count FROM filtered
)
SELECT filtered.*, total.total_count
FROM filtered CROSS JOIN total
ORDER BY filtered.created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: UpdateUserByUUID :one
UPDATE users SET
    name = $2,
    email = $3,
    disabled = $4,
    updated_at = now()
WHERE uuid = $1
RETURNING *;

-- name: SetUserPasswordHashByID :exec
UPDATE users SET
    password_hash = $2,
    updated_at = now()
WHERE id = $1;

-- name: SetUserLastLoginByID :exec
UPDATE users SET
    last_login_at = now(),
    updated_at = now()
WHERE id = $1;

-- name: DeleteUserByUUID :execrows
DELETE FROM users WHERE uuid = $1;
