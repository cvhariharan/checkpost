-- name: CreateSavedQuery :one
INSERT INTO saved_queries (
    name,
    description,
    query,
    targets,
    visibility,
    created_by
) VALUES (
    $1, $2, $3, sqlc.arg(targets)::text::jsonb, $4, $5
)
RETURNING *;

-- name: GetSavedQueryByUUID :one
SELECT saved_queries.*, users.name AS creator_name
FROM saved_queries
LEFT JOIN users ON users.id = saved_queries.created_by
WHERE saved_queries.uuid = $1;

-- name: CountSavedQueries :one
SELECT count(*)
FROM saved_queries
WHERE (visibility = 'public' OR created_by = @viewer_id)
  AND (
    @query::text = ''
    OR name ILIKE '%' || @query::text || '%'
    OR description ILIKE '%' || @query::text || '%'
);

-- name: ListSavedQueries :many
SELECT saved_queries.*, users.name AS creator_name
FROM saved_queries
LEFT JOIN users ON users.id = saved_queries.created_by
WHERE (saved_queries.visibility = 'public' OR saved_queries.created_by = @viewer_id)
  AND (
    @query::text = ''
    OR saved_queries.name ILIKE '%' || @query::text || '%'
    OR saved_queries.description ILIKE '%' || @query::text || '%'
)
ORDER BY saved_queries.created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: UpdateSavedQueryByUUID :one
UPDATE saved_queries
SET
    name = $2,
    description = $3,
    query = $4,
    targets = sqlc.arg(targets)::text::jsonb,
    visibility = $5,
    updated_at = now()
WHERE uuid = $1
RETURNING *;

-- name: DeleteSavedQueryByUUID :execrows
DELETE FROM saved_queries WHERE uuid = @saved_query_uuid;
