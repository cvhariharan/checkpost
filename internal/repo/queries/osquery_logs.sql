-- name: CreateResultBatch :one
INSERT INTO osquery_result_batches (
    node_id,
    schedule_name,
    action,
    calendar_time,
    counter,
    epoch,
    numerics,
    unix_time,
    is_system
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: CreateResultRow :one
INSERT INTO osquery_result_rows (batch_id, row_index)
VALUES ($1, $2)
RETURNING *;

-- name: CreateResultCell :exec
INSERT INTO osquery_result_cells (row_id, column_name, value_text)
VALUES ($1, $2, $3);

-- name: CreateStatusLog :one
INSERT INTO osquery_status_logs (
    node_id,
    calendar_time,
    file_name,
    line,
    message,
    severity,
    unix_time,
    version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;
