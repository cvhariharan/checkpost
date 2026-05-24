-- name: GetQuerySchema :one
SELECT schedule_uuid, sql_version, columns, first_observed_at, last_observed_at, row_count_estimate
FROM query_schemas
WHERE schedule_uuid = $1 AND sql_version = $2;

-- name: UpsertQuerySchema :one
-- Atomically merges new columns into the persisted set. Concurrent writers
-- observing different new columns for the same (schedule_uuid, sql_version)
-- would otherwise overwrite each other's column lists; doing the merge in
-- a single UPDATE keeps the additions strictly cumulative because the
-- statement reads the current row value under the row lock acquired by
-- ON CONFLICT DO UPDATE.
INSERT INTO query_schemas (schedule_uuid, sql_version, columns)
VALUES ($1, $2, $3)
ON CONFLICT (schedule_uuid, sql_version) DO UPDATE
SET columns = query_schemas.columns || (
        SELECT COALESCE(jsonb_agg(c), '[]'::jsonb)
        FROM jsonb_array_elements_text(EXCLUDED.columns) AS c
        WHERE NOT (query_schemas.columns ? c)
    ),
    last_observed_at = now()
RETURNING schedule_uuid, sql_version, columns, first_observed_at, last_observed_at, row_count_estimate;

-- name: UpdateQuerySchemaRowCount :exec
UPDATE query_schemas
SET row_count_estimate = $3,
    last_observed_at = now()
WHERE schedule_uuid = $1 AND sql_version = $2;

-- name: ListQuerySchemasForSchedule :many
SELECT schedule_uuid, sql_version, columns, first_observed_at, last_observed_at, row_count_estimate
FROM query_schemas
WHERE schedule_uuid = $1
ORDER BY sql_version DESC;

-- name: ListAllQuerySchemas :many
SELECT schedule_uuid, sql_version
FROM query_schemas
ORDER BY schedule_uuid, sql_version;

-- name: DeleteQuerySchema :exec
DELETE FROM query_schemas
WHERE schedule_uuid = $1 AND sql_version = $2;

-- name: DeleteQuerySchemasForSchedule :exec
DELETE FROM query_schemas
WHERE schedule_uuid = $1;
