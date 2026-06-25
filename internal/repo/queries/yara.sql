-- name: CreateYaraSignatureSource :one
INSERT INTO yara_signature_sources (
    group_id,
    url,
    label,
    enabled
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: UpdateYaraSignatureSourceByUUID :one
UPDATE yara_signature_sources SET
    group_id = $2,
    url = $3,
    label = $4,
    enabled = $5,
    updated_at = now()
WHERE uuid = $1
RETURNING *;

-- name: DeleteYaraSignatureSourceByUUID :execrows
DELETE FROM yara_signature_sources WHERE uuid = $1;

-- name: GetYaraSignatureSourceByUUID :one
SELECT * FROM yara_signature_sources WHERE uuid = $1;

-- name: ListYaraSignatureSources :many
WITH filtered AS (
    SELECT
        yara_signature_sources.id,
        yara_signature_sources.uuid,
        yara_signature_sources.group_id,
        groups.uuid AS group_uuid,
        groups.name AS group_name,
        yara_signature_sources.url,
        yara_signature_sources.label,
        yara_signature_sources.enabled,
        yara_signature_sources.created_at,
        yara_signature_sources.updated_at
    FROM yara_signature_sources
    LEFT JOIN groups ON groups.id = yara_signature_sources.group_id
),
total AS (
    SELECT count(*) AS total_count FROM filtered
)
SELECT filtered.*, total.total_count
FROM filtered, total
ORDER BY filtered.group_name NULLS FIRST, filtered.created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: ListEnabledDefaultYaraSignatureSources :many
SELECT *
FROM yara_signature_sources
WHERE group_id IS NULL
  AND enabled = true
ORDER BY created_at ASC;

-- name: ListEnabledYaraSignatureSourcesByGroupID :many
SELECT *
FROM yara_signature_sources
WHERE group_id = $1
  AND enabled = true
ORDER BY created_at ASC;

-- name: CreateYaraScan :one
INSERT INTO yara_scans (
    group_id,
    paths,
    rule_urls,
    target_count
) VALUES (
    sqlc.arg(group_id), sqlc.arg(paths)::text[], sqlc.arg(rule_urls)::text[], sqlc.arg(target_count)
)
RETURNING *;

-- name: CreateYaraScanTarget :exec
INSERT INTO yara_scan_targets (
    scan_id,
    node_id
) VALUES (
    $1, $2
)
ON CONFLICT (scan_id, node_id) DO NOTHING;

-- name: ListYaraScans :many
WITH filtered AS (
    SELECT
        yara_scans.id,
        yara_scans.uuid,
        yara_scans.group_id,
        groups.uuid AS group_uuid,
        groups.name AS group_name,
        yara_scans.paths,
        yara_scans.status,
        yara_scans.target_count,
        yara_scans.completed_count,
        yara_scans.match_count,
        yara_scans.error,
        yara_scans.created_at,
        yara_scans.updated_at,
        yara_scans.completed_at
    FROM yara_scans
    LEFT JOIN groups ON groups.id = yara_scans.group_id
),
total AS (
    SELECT count(*) AS total_count FROM filtered
)
SELECT filtered.*, total.total_count
FROM filtered, total
ORDER BY filtered.created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: GetYaraScanByUUID :one
SELECT
    yara_scans.id,
    yara_scans.uuid,
    yara_scans.group_id,
    groups.uuid AS group_uuid,
    groups.name AS group_name,
    yara_scans.paths,
    yara_scans.status,
    yara_scans.target_count,
    yara_scans.completed_count,
    yara_scans.match_count,
    yara_scans.error,
    yara_scans.created_at,
    yara_scans.updated_at,
    yara_scans.completed_at
FROM yara_scans
LEFT JOIN groups ON groups.id = yara_scans.group_id
WHERE yara_scans.uuid = $1;

-- name: TimeoutStaleYaraScanTargets :many
UPDATE yara_scan_targets
SET status = 'error',
    error = CASE
        WHEN status = 'pending' THEN 'YARA scan was not dispatched before timeout'
        ELSE 'YARA scan did not report results before timeout'
    END,
    completed_at = now(),
    updated_at = now()
WHERE status IN ('pending', 'dispatched')
  AND COALESCE(dispatched_at, created_at) < now() - sqlc.arg(timeout)::text::interval
RETURNING scan_id;

-- name: ListPendingYaraScanTargetsForNode :many
SELECT
    yara_scans.id AS scan_id,
    yara_scans.uuid AS scan_uuid,
    yara_scans.paths,
    yara_scans.rule_urls
FROM yara_scan_targets
JOIN yara_scans ON yara_scans.id = yara_scan_targets.scan_id
WHERE yara_scan_targets.node_id = $1
  AND yara_scan_targets.status = 'pending'
ORDER BY yara_scan_targets.created_at ASC;

-- name: MarkYaraScanTargetDispatched :exec
UPDATE yara_scan_targets
SET status = 'dispatched',
    dispatched_at = COALESCE(dispatched_at, now()),
    updated_at = now()
WHERE scan_id = $1
  AND node_id = $2
  AND status = 'pending';

-- name: MarkYaraScanRunning :exec
UPDATE yara_scans
SET status = 'running',
    updated_at = now()
WHERE id = $1
  AND status = 'pending';

-- name: InsertYaraScanMatch :exec
INSERT INTO yara_scan_matches (
    scan_id,
    node_id,
    path,
    matches,
    count
) VALUES (
    $1, $2, $3, $4, $5
);

-- name: CompleteYaraScanTarget :exec
UPDATE yara_scan_targets
SET status = 'complete',
    error = '',
    completed_at = now(),
    updated_at = now()
WHERE scan_id = $1
  AND node_id = $2;

-- name: ErrorYaraScanTarget :exec
UPDATE yara_scan_targets
SET status = 'error',
    error = $3,
    completed_at = now(),
    updated_at = now()
WHERE scan_id = $1
  AND node_id = $2;

-- name: RefreshYaraScanStats :exec
WITH stats AS (
    SELECT
        yara_scan_targets.scan_id,
        count(*)::integer AS target_count,
        count(*) FILTER (WHERE status IN ('complete', 'error'))::integer AS completed_count,
        count(*) FILTER (WHERE status = 'error')::integer AS error_count
    FROM yara_scan_targets
    WHERE yara_scan_targets.scan_id = $1
    GROUP BY yara_scan_targets.scan_id
),
matches AS (
    SELECT yara_scan_matches.scan_id, count(*)::integer AS match_count
    FROM yara_scan_matches
    WHERE yara_scan_matches.scan_id = $1
    GROUP BY yara_scan_matches.scan_id
),
first_error AS (
    SELECT scan_id, error
    FROM yara_scan_targets
    WHERE scan_id = $1
      AND status = 'error'
      AND length(trim(error)) > 0
    ORDER BY completed_at ASC NULLS LAST, updated_at ASC
    LIMIT 1
)
UPDATE yara_scans
SET target_count = stats.target_count,
    completed_count = stats.completed_count,
    match_count = COALESCE(matches.match_count, 0),
    status = CASE
        WHEN stats.completed_count = 0 AND yara_scans.status = 'pending' THEN 'pending'
        WHEN stats.completed_count < stats.target_count THEN 'running'
        WHEN stats.error_count = 0 THEN 'complete'
        WHEN stats.error_count = stats.target_count THEN 'error'
        ELSE 'partial'
    END,
    error = CASE
        WHEN stats.error_count = 0 THEN ''
        WHEN stats.error_count = 1 THEN first_error.error
        ELSE stats.error_count::text || ' YARA scan targets failed; first error: ' || COALESCE(first_error.error, 'unknown error')
    END,
    completed_at = CASE
        WHEN stats.completed_count = stats.target_count THEN COALESCE(yara_scans.completed_at, now())
        ELSE NULL
    END,
    updated_at = now()
FROM stats
LEFT JOIN matches ON matches.scan_id = stats.scan_id
LEFT JOIN first_error ON first_error.scan_id = stats.scan_id
WHERE yara_scans.id = stats.scan_id;

-- name: ListYaraScanMatches :many
WITH filtered AS (
    SELECT
        yara_scan_matches.id,
        nodes.uuid AS node_uuid,
        COALESCE(NULLIF(nodes.display_name, ''), nodes.hostname) AS hostname,
        yara_scan_matches.path,
        yara_scan_matches.matches,
        yara_scan_matches.count,
        yara_scan_matches.created_at
    FROM yara_scan_matches
    JOIN nodes ON nodes.id = yara_scan_matches.node_id
    JOIN yara_scans ON yara_scans.id = yara_scan_matches.scan_id
    WHERE yara_scans.uuid = @scan_uuid
),
total AS (
    SELECT count(*) AS total_count FROM filtered
)
SELECT filtered.*, total.total_count
FROM filtered, total
ORDER BY filtered.created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: ListYaraScanTargets :many
WITH filtered AS (
    SELECT
        nodes.uuid AS node_uuid,
        COALESCE(NULLIF(nodes.display_name, ''), nodes.hostname) AS hostname,
        yara_scan_targets.status,
        yara_scan_targets.dispatched_at,
        yara_scan_targets.completed_at,
        yara_scan_targets.error,
        yara_scan_targets.created_at,
        yara_scan_targets.updated_at
    FROM yara_scan_targets
    JOIN nodes ON nodes.id = yara_scan_targets.node_id
    JOIN yara_scans ON yara_scans.id = yara_scan_targets.scan_id
    WHERE yara_scans.uuid = @scan_uuid
),
total AS (
    SELECT count(*) AS total_count FROM filtered
)
SELECT filtered.*, total.total_count
FROM filtered, total
ORDER BY filtered.created_at ASC
LIMIT @limit_count OFFSET @offset_count;

-- name: ListAllNodeIDs :many
SELECT id FROM nodes ORDER BY id;

-- name: ListNodeIDsByGroupUUID :many
SELECT nodes.id
FROM nodes
JOIN group_membership ON group_membership.node_id = nodes.id
JOIN groups ON groups.id = group_membership.group_id
WHERE groups.uuid = $1
ORDER BY nodes.id;
