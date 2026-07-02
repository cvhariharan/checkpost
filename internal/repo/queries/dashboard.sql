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

-- Fibonacci severity weights
-- name: DashboardPolicyRowCounts :one
SELECT
    count(*) FILTER (WHERE passes = true AND checked_at >= @stale_cutoff::timestamptz)::bigint AS passing,
    count(*) FILTER (WHERE passes = false AND checked_at >= @stale_cutoff::timestamptz)::bigint AS failing,
    count(*) FILTER (WHERE passes IS NULL OR checked_at < @stale_cutoff::timestamptz)::bigint AS unknown,
    coalesce(sum(
        CASE policies.severity
            WHEN 'critical' THEN 8 WHEN 'high' THEN 5 WHEN 'medium' THEN 3 WHEN 'low' THEN 2 WHEN 'info' THEN 1 ELSE 3
        END
    ) FILTER (WHERE passes = true AND checked_at >= @stale_cutoff::timestamptz), 0)::bigint AS weighted_passing,
    coalesce(sum(
        CASE policies.severity
            WHEN 'critical' THEN 8 WHEN 'high' THEN 5 WHEN 'medium' THEN 3 WHEN 'low' THEN 2 WHEN 'info' THEN 1 ELSE 3
        END
    ), 0)::bigint AS weighted_total
FROM policy_membership
JOIN policies ON policies.id = policy_membership.policy_id;

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
        nodes.id AS node_id,
        count(*) FILTER (WHERE policy_membership.passes = true AND policy_membership.checked_at >= @stale_cutoff::timestamptz)::bigint AS passing,
        count(*) FILTER (WHERE policy_membership.passes = false AND policy_membership.checked_at >= @stale_cutoff::timestamptz)::bigint AS failing,
        count(*)::bigint AS total,
        coalesce(sum(
            CASE policies.severity
                WHEN 'critical' THEN 8 WHEN 'high' THEN 5 WHEN 'medium' THEN 3 WHEN 'low' THEN 2 WHEN 'info' THEN 1 ELSE 3
            END
        ) FILTER (WHERE policy_membership.passes = true AND policy_membership.checked_at >= @stale_cutoff::timestamptz), 0)::bigint AS weighted_passing,
        coalesce(sum(
            CASE policies.severity
                WHEN 'critical' THEN 8 WHEN 'high' THEN 5 WHEN 'medium' THEN 3 WHEN 'low' THEN 2 WHEN 'info' THEN 1 ELSE 3
            END
        ), 0)::bigint AS weighted_total
    FROM nodes
    JOIN policies ON policies.enabled = true
        AND (
            policies.platform IN ('all', 'any')
            OR policies.platform = nodes.platform
            OR (policies.platform = 'linux' AND nodes.platform NOT IN ('', 'darwin', 'windows'))
            OR (policies.platform = 'posix' AND nodes.platform NOT IN ('', 'windows'))
        )
        AND (
            NOT EXISTS (
                SELECT 1
                FROM policy_groups
                WHERE policy_groups.policy_id = policies.id
            )
            OR EXISTS (
                SELECT 1
                FROM policy_groups
                JOIN group_membership ON group_membership.group_id = policy_groups.group_id
                WHERE policy_groups.policy_id = policies.id
                  AND group_membership.node_id = nodes.id
            )
        )
    LEFT JOIN policy_membership
        ON policy_membership.policy_id = policies.id
       AND policy_membership.node_id = nodes.id
    GROUP BY nodes.id
)
SELECT
    nodes.uuid,
    nodes.hostname,
    nodes.display_name,
    per_node.passing,
    per_node.failing,
    per_node.total,
    per_node.weighted_passing,
    per_node.weighted_total
FROM per_node
JOIN nodes ON nodes.id = per_node.node_id
ORDER BY per_node.weighted_passing::numeric / nullif(per_node.weighted_total, 0) ASC, per_node.failing DESC, nodes.hostname ASC
LIMIT @top_n;

-- name: DashboardMostCompliantNodes :many
WITH per_node AS (
    SELECT
        nodes.id AS node_id,
        count(*) FILTER (WHERE policy_membership.passes = true AND policy_membership.checked_at >= @stale_cutoff::timestamptz)::bigint AS passing,
        count(*) FILTER (WHERE policy_membership.passes = false AND policy_membership.checked_at >= @stale_cutoff::timestamptz)::bigint AS failing,
        count(*)::bigint AS total,
        coalesce(sum(
            CASE policies.severity
                WHEN 'critical' THEN 8 WHEN 'high' THEN 5 WHEN 'medium' THEN 3 WHEN 'low' THEN 2 WHEN 'info' THEN 1 ELSE 3
            END
        ) FILTER (WHERE policy_membership.passes = true AND policy_membership.checked_at >= @stale_cutoff::timestamptz), 0)::bigint AS weighted_passing,
        coalesce(sum(
            CASE policies.severity
                WHEN 'critical' THEN 8 WHEN 'high' THEN 5 WHEN 'medium' THEN 3 WHEN 'low' THEN 2 WHEN 'info' THEN 1 ELSE 3
            END
        ), 0)::bigint AS weighted_total
    FROM nodes
    JOIN policies ON policies.enabled = true
        AND (
            policies.platform IN ('all', 'any')
            OR policies.platform = nodes.platform
            OR (policies.platform = 'linux' AND nodes.platform NOT IN ('', 'darwin', 'windows'))
            OR (policies.platform = 'posix' AND nodes.platform NOT IN ('', 'windows'))
        )
        AND (
            NOT EXISTS (
                SELECT 1
                FROM policy_groups
                WHERE policy_groups.policy_id = policies.id
            )
            OR EXISTS (
                SELECT 1
                FROM policy_groups
                JOIN group_membership ON group_membership.group_id = policy_groups.group_id
                WHERE policy_groups.policy_id = policies.id
                  AND group_membership.node_id = nodes.id
            )
        )
    LEFT JOIN policy_membership
        ON policy_membership.policy_id = policies.id
       AND policy_membership.node_id = nodes.id
    GROUP BY nodes.id
)
SELECT
    nodes.uuid,
    nodes.hostname,
    nodes.display_name,
    per_node.passing,
    per_node.failing,
    per_node.total,
    per_node.weighted_passing,
    per_node.weighted_total
FROM per_node
JOIN nodes ON nodes.id = per_node.node_id
ORDER BY per_node.weighted_passing::numeric / nullif(per_node.weighted_total, 0) DESC, per_node.failing ASC, nodes.hostname ASC
LIMIT @top_n;

-- name: DashboardFiringAlertsBySeverity :many
SELECT
    alert_rules.severity,
    count(*)::bigint AS count
FROM alert_state
JOIN alert_rules ON alert_rules.id = alert_state.rule_id
WHERE alert_state.status = 'firing'
GROUP BY alert_rules.severity;

-- name: DashboardFiringAlerts :many
SELECT
    alert_rules.uuid,
    alert_rules.name,
    alert_rules.severity,
    count(*)::bigint AS count,
    max(alert_state.last_seen_at)::timestamptz AS last_seen_at
FROM alert_state
JOIN alert_rules ON alert_rules.id = alert_state.rule_id
WHERE alert_state.status = 'firing'
GROUP BY alert_rules.uuid, alert_rules.name, alert_rules.severity
ORDER BY
    CASE alert_rules.severity
        WHEN 'critical' THEN 0
        WHEN 'high' THEN 1
        WHEN 'medium' THEN 2
        WHEN 'low' THEN 3
        WHEN 'info' THEN 4
        ELSE 5
    END,
    last_seen_at DESC
LIMIT @top_n;

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
