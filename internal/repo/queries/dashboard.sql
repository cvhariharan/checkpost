-- name: DashboardNodeCounts :one
SELECT
    count(*)::bigint AS total,
    count(*) FILTER (
        WHERE last_seen_at IS NOT NULL AND last_seen_at >= @online_cutoff::timestamptz
    )::bigint AS online,
    count(*) FILTER (WHERE last_seen_at IS NULL)::bigint AS never_reported
FROM nodes;

-- name: DashboardNodeCountsByPlatform :many
SELECT
    platform,
    count(*)::bigint AS total,
    count(*) FILTER (
        WHERE last_seen_at IS NOT NULL AND last_seen_at >= @online_cutoff::timestamptz
    )::bigint AS online
FROM nodes
GROUP BY platform
ORDER BY total DESC, platform;

-- name: DashboardPolicyRowCounts :one
SELECT
    count(*) FILTER (WHERE passes = true AND checked_at >= @stale_cutoff::timestamptz)::bigint AS passing,
    count(*) FILTER (WHERE passes = false AND checked_at >= @stale_cutoff::timestamptz)::bigint AS failing,
    count(*) FILTER (WHERE passes IS NULL OR checked_at < @stale_cutoff::timestamptz)::bigint AS unknown
FROM policy_membership;

-- name: DashboardMachineComplianceCounts :one
WITH per_node AS (
    SELECT
        node_id,
        bool_or(passes = false AND checked_at >= @stale_cutoff::timestamptz) AS has_failing,
        bool_or(passes IS NULL OR checked_at < @stale_cutoff::timestamptz) AS has_unknown
    FROM policy_membership
    GROUP BY node_id
)
SELECT
    count(*) FILTER (WHERE has_failing)::bigint AS failing,
    count(*) FILTER (WHERE NOT has_failing AND has_unknown)::bigint AS unknown,
    count(*) FILTER (WHERE NOT has_failing AND NOT has_unknown)::bigint AS passing,
    ((SELECT count(*) FROM nodes) - count(*))::bigint AS no_policies
FROM per_node;

-- name: DashboardTopFailingPolicies :many
SELECT
    policies.uuid,
    policies.name,
    policies.platform,
    count(*) FILTER (
        WHERE policy_membership.passes = false AND policy_membership.checked_at >= @stale_cutoff::timestamptz
    )::bigint AS failing_count
FROM policies
JOIN policy_membership ON policy_membership.policy_id = policies.id
GROUP BY policies.id, policies.uuid, policies.name, policies.platform
HAVING count(*) FILTER (
    WHERE policy_membership.passes = false AND policy_membership.checked_at >= @stale_cutoff::timestamptz
) > 0
ORDER BY failing_count DESC, policies.name
LIMIT @top_n;

-- name: DashboardLeastCompliantNodes :many
WITH per_node AS (
    SELECT
        node_id,
        count(*) FILTER (WHERE passes = true AND checked_at >= @stale_cutoff::timestamptz)::bigint AS passing,
        count(*) FILTER (WHERE passes = false AND checked_at >= @stale_cutoff::timestamptz)::bigint AS failing,
        count(*)::bigint AS total
    FROM policy_membership
    GROUP BY node_id
)
SELECT
    nodes.uuid,
    nodes.hostname,
    nodes.display_name,
    per_node.passing,
    per_node.failing,
    per_node.total
FROM per_node
JOIN nodes ON nodes.id = per_node.node_id
ORDER BY per_node.passing::numeric / per_node.total ASC, per_node.failing DESC, nodes.hostname ASC
LIMIT @top_n;

-- name: DashboardMostCompliantNodes :many
WITH per_node AS (
    SELECT
        node_id,
        count(*) FILTER (WHERE passes = true AND checked_at >= @stale_cutoff::timestamptz)::bigint AS passing,
        count(*) FILTER (WHERE passes = false AND checked_at >= @stale_cutoff::timestamptz)::bigint AS failing,
        count(*)::bigint AS total
    FROM policy_membership
    GROUP BY node_id
)
SELECT
    nodes.uuid,
    nodes.hostname,
    nodes.display_name,
    per_node.passing,
    per_node.failing,
    per_node.total
FROM per_node
JOIN nodes ON nodes.id = per_node.node_id
ORDER BY per_node.passing::numeric / per_node.total DESC, per_node.failing ASC, nodes.hostname ASC
LIMIT @top_n;

-- name: DashboardFiringAlertsBySeverity :many
SELECT
    alert_rules.severity,
    count(*)::bigint AS count
FROM alert_state
JOIN alert_rules ON alert_rules.id = alert_state.rule_id
WHERE alert_state.status = 'firing'
GROUP BY alert_rules.severity;

-- name: DashboardRecentYaraMatches :many
SELECT
    yara_scans.uuid AS scan_uuid,
    nodes.uuid AS node_uuid,
    nodes.hostname,
    nodes.display_name,
    yara_scan_matches.path,
    yara_scan_matches.matches,
    yara_scan_matches.created_at
FROM yara_scan_matches
JOIN yara_scans ON yara_scans.id = yara_scan_matches.scan_id
JOIN nodes ON nodes.id = yara_scan_matches.node_id
ORDER BY yara_scan_matches.created_at DESC
LIMIT @top_n;

-- name: DashboardRecentEnrollments :many
SELECT uuid, hostname, display_name, enrolled_at
FROM nodes
ORDER BY enrolled_at DESC
LIMIT @top_n;
