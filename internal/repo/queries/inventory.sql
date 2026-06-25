-- name: CreateDeviceOwner :one
INSERT INTO device_owners (
    display_name,
    email,
    external_id,
    department,
    title,
    phone,
    notes
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: GetDeviceOwnerByUUID :one
SELECT * FROM device_owners WHERE uuid = $1;

-- name: GetDeviceOwnerWithCountsByUUID :one
SELECT
    device_owners.id,
    device_owners.uuid,
    device_owners.display_name,
    device_owners.email,
    device_owners.external_id,
    device_owners.department,
    device_owners.title,
    device_owners.phone,
    device_owners.notes,
    device_owners.created_at,
    device_owners.updated_at,
    count(node_inventory.node_id)::bigint AS machine_count
FROM device_owners
LEFT JOIN node_inventory ON node_inventory.owner_id = device_owners.id
WHERE device_owners.uuid = @owner_uuid
GROUP BY
    device_owners.id,
    device_owners.uuid,
    device_owners.display_name,
    device_owners.email,
    device_owners.external_id,
    device_owners.department,
    device_owners.title,
    device_owners.phone,
    device_owners.notes,
    device_owners.created_at,
    device_owners.updated_at;

-- name: ListDeviceOwnersWithCounts :many
WITH filtered AS (
    SELECT *
    FROM device_owners
    WHERE (
        @query::text = ''
        OR display_name ILIKE '%' || @query::text || '%'
        OR email ILIKE '%' || @query::text || '%'
    )
),
total AS (
    SELECT count(*) AS total_count FROM filtered
)
SELECT
    filtered.id,
    filtered.uuid,
    filtered.display_name,
    filtered.email,
    filtered.external_id,
    filtered.department,
    filtered.title,
    filtered.phone,
    filtered.notes,
    filtered.created_at,
    filtered.updated_at,
    count(node_inventory.node_id)::bigint AS machine_count,
    total.total_count
FROM filtered
CROSS JOIN total
LEFT JOIN node_inventory ON node_inventory.owner_id = filtered.id
GROUP BY
    filtered.id,
    filtered.uuid,
    filtered.display_name,
    filtered.email,
    filtered.external_id,
    filtered.department,
    filtered.title,
    filtered.phone,
    filtered.notes,
    filtered.created_at,
    filtered.updated_at,
    total.total_count
ORDER BY filtered.display_name ASC, filtered.created_at DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: UpdateDeviceOwnerByUUID :one
UPDATE device_owners SET
    display_name = $2,
    email = $3,
    external_id = $4,
    department = $5,
    title = $6,
    phone = $7,
    notes = $8,
    updated_at = now()
WHERE uuid = $1
RETURNING *;

-- name: DeleteDeviceOwnerByUUID :execrows
DELETE FROM device_owners WHERE uuid = $1;

-- name: GetNodeInventoryByNodeUUID :one
SELECT
    node_inventory.node_id,
    node_inventory.owner_id,
    node_inventory.internal_tracking_id,
    node_inventory.notes,
    node_inventory.created_at,
    node_inventory.updated_at,
    device_owners.id AS owner_db_id,
    device_owners.uuid AS owner_uuid,
    device_owners.display_name AS owner_display_name,
    device_owners.email AS owner_email,
    device_owners.external_id AS owner_external_id,
    device_owners.department AS owner_department,
    device_owners.title AS owner_title,
    device_owners.phone AS owner_phone,
    device_owners.notes AS owner_notes,
    device_owners.created_at AS owner_created_at,
    device_owners.updated_at AS owner_updated_at
FROM node_inventory
JOIN nodes ON nodes.id = node_inventory.node_id
LEFT JOIN device_owners ON device_owners.id = node_inventory.owner_id
WHERE nodes.uuid = @node_uuid;

-- name: ListNodeInventoriesByNodeUUIDs :many
SELECT
    nodes.uuid AS node_uuid,
    node_inventory.node_id,
    node_inventory.owner_id,
    node_inventory.internal_tracking_id,
    node_inventory.notes,
    node_inventory.created_at,
    node_inventory.updated_at,
    device_owners.id AS owner_db_id,
    device_owners.uuid AS owner_uuid,
    device_owners.display_name AS owner_display_name,
    device_owners.email AS owner_email,
    device_owners.external_id AS owner_external_id,
    device_owners.department AS owner_department,
    device_owners.title AS owner_title,
    device_owners.phone AS owner_phone,
    device_owners.notes AS owner_notes,
    device_owners.created_at AS owner_created_at,
    device_owners.updated_at AS owner_updated_at
FROM nodes
JOIN node_inventory ON node_inventory.node_id = nodes.id
LEFT JOIN device_owners ON device_owners.id = node_inventory.owner_id
WHERE nodes.uuid = ANY(@node_uuids::uuid[]);

-- name: UpsertNodeInventory :one
INSERT INTO node_inventory (
    node_id,
    owner_id,
    internal_tracking_id,
    notes
) VALUES (
    $1, $2, $3, $4
)
ON CONFLICT (node_id) DO UPDATE SET
    owner_id = EXCLUDED.owner_id,
    internal_tracking_id = EXCLUDED.internal_tracking_id,
    notes = EXCLUDED.notes,
    updated_at = now()
RETURNING *;

-- name: DeleteNodeInventoryByNodeUUID :execrows
DELETE FROM node_inventory
USING nodes
WHERE node_inventory.node_id = nodes.id
  AND nodes.uuid = @node_uuid;

-- name: ListNodesByOwner :many
WITH target_owner AS (
    SELECT *
    FROM device_owners
    WHERE device_owners.uuid = @owner_uuid
),
filtered AS (
    SELECT
        nodes.id,
        nodes.uuid,
        nodes.node_key,
        nodes.host_identifier,
        nodes.hostname,
        nodes.display_name,
        nodes.platform,
        nodes.os_name,
        nodes.os_version,
        nodes.osquery_version,
        nodes.hardware_serial,
        nodes.enrolled_at,
        nodes.last_seen_at,
        nodes.last_policy_check_at,
        nodes.created_at,
        nodes.updated_at
    FROM target_owner
    JOIN node_inventory ON node_inventory.owner_id = target_owner.id
    JOIN nodes ON nodes.id = node_inventory.node_id
),
total AS (
    SELECT count(*) AS total_count FROM filtered
)
SELECT filtered.*, total.total_count
FROM filtered, total
ORDER BY filtered.hostname, filtered.created_at DESC
LIMIT @limit_count OFFSET @offset_count;
