-- name: CreateQuery :one
INSERT INTO queries (name, sql, description, is_system)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetQueryByUUID :one
SELECT * FROM queries WHERE uuid = $1;

-- name: GetQueryByID :one
SELECT * FROM queries WHERE id = $1;

-- name: ListQueries :many
WITH filtered AS (
    SELECT * FROM queries
),
total AS (
    SELECT count(*) AS total_count FROM filtered
)
SELECT filtered.*, total.total_count
FROM filtered, total
ORDER BY filtered.created_at DESC
LIMIT $1 OFFSET $2;

-- name: UpdateQueryByUUID :one
UPDATE queries SET
    name = $2,
    sql = $3,
    description = $4,
    updated_at = now()
WHERE uuid = $1 AND is_system = false
RETURNING *;

-- name: DeleteQueryByUUID :execrows
DELETE FROM queries WHERE uuid = $1 AND is_system = false;
