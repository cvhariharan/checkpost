-- name: UpsertNodeMetric :exec
INSERT INTO node_metrics (node_id, kind, value, collected_at, updated_at)
VALUES ($1, $2, $3, $4, now())
ON CONFLICT (node_id, kind) DO UPDATE SET
    value = EXCLUDED.value,
    collected_at = EXCLUDED.collected_at,
    updated_at = now();

-- name: ListNodeMetricsByNodeUUID :many
SELECT m.kind, m.value, m.collected_at, m.updated_at
FROM node_metrics m
JOIN nodes n ON n.id = m.node_id
WHERE n.uuid = $1
ORDER BY m.kind;
