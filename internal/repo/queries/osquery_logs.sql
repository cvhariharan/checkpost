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
