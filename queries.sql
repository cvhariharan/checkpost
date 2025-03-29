-- name: add-node
INSERT INTO
    nodes (host_identifier, os_name)
VALUES
    ($1, $2) RETURNING id, uuid;

-- name: get-node-by-uuid
WITH node AS (
    SELECT
        *
    FROM
        nodes
    WHERE
        nodes.uuid = $1
) SELECT
    node.uuid,
    node.host_identifier,

    -- OS Version Info
    json_build_object(
        'uuid', os_version_info.uuid,
        'os_id', os_version_info.os_id,
        'codename', os_version_info.codename,
        'major', os_version_info.major,
        'minor', os_version_info.minor,
        'name', os_version_info.name,
        'patch', os_version_info.patch,
        'platform', os_version_info.platform,
        'platform_like', os_version_info.platform_like,
        'version', os_version_info.version
    ) AS os_version,

    -- OSQuery Info
    json_build_object(
        'uuid', osquery_info.uuid,
        'build_distro', osquery_info.build_distro,
        'build_platform', osquery_info.build_platform,
        'config_hash', osquery_info.config_hash,
        'config_valid', osquery_info.config_valid,
        'extensions', osquery_info.extensions,
        'instance_id', osquery_info.instance_id,
        'pid', osquery_info.pid,
        'start_time', osquery_info.start_time,
        'version', osquery_info.version,
        'watcher', osquery_info.watcher
    ) AS osquery_info,

    -- System Info
    json_build_object(
        'uuid', system_info.uuid,
        'computer_name', system_info.computer_name,
        'cpu_brand', system_info.cpu_brand,
        'cpu_logical_cores', system_info.cpu_logical_cores,
        'cpu_physical_cores', system_info.cpu_physical_cores,
        'cpu_subtype', system_info.cpu_subtype,
        'cpu_type', system_info.cpu_type,
        'hardware_model', system_info.hardware_model,
        'hardware_serial', system_info.hardware_serial,
        'hardware_vendor', system_info.hardware_vendor,
        'hardware_version', system_info.hardware_version,
        'hostname', system_info.hostname,
        'local_hostname', system_info.local_hostname,
        'physical_memory', system_info.physical_memory
    ) AS system_info,

    -- Platform Info
    json_build_object(
        'uuid', platform_info.uuid,
        'address', platform_info.address,
        'date', platform_info.date,
        'extra', platform_info.extra,
        'revision', platform_info.revision,
        'size', platform_info.size,
        'vendor', platform_info.vendor,
        'version', platform_info.version,
        'volume_size', platform_info.volume_size
    ) AS platform_info

FROM
    node
LEFT JOIN os_version_info ON node.id = os_version_info.node_fk
LEFT JOIN osquery_info ON node.id = osquery_info.node_fk
LEFT JOIN system_info ON node.id = system_info.node_fk
LEFT JOIN platform_info ON node.id = platform_info.node_fk;

-- name: add-os-version-info
INSERT INTO
    os_version_info (
        os_id,
        codename,
        major,
        minor,
        name,
        patch,
        platform,
        platform_like,
        version,
        node_fk
    )
VALUES
    ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING *;

-- name: get-os-version-info
SELECT * FROM os_version_info WHERE node_fk = $1;

-- name: add-osquery-info
INSERT INTO
    osquery_info (
        build_distro,
        build_platform,
        config_hash,
        config_valid,
        extensions,
        instance_id,
        pid,
        start_time,
        uuid,
        version,
        watcher,
        node_fk
    )
VALUES
    ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) RETURNING *;

-- name: get-osquery-info
SELECT * FROM osquery_info WHERE node_fk = $1;

-- name: add-system-info
INSERT INTO
    system_info (
        computer_name,
        cpu_brand,
        cpu_logical_cores,
        cpu_physical_cores,
        cpu_subtype,
        cpu_type,
        hardware_model,
        hardware_serial,
        hardware_vendor,
        hardware_version,
        hostname,
        local_hostname,
        physical_memory,
        uuid,
        node_fk
    )
VALUES
    ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15) RETURNING *;

-- name: get-system-info
SELECT * FROM system_info WHERE node_fk = $1;

-- name: add-platform-info
INSERT INTO
    platform_info (
        address,
        date,
        extra,
        revision,
        size,
        vendor,
        version,
        volume_size,
        node_fk
    )
VALUES
    ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING *;

-- name: get-platform-info
SELECT * FROM platform_info WHERE node_fk = $1;

-- name: create-query
INSERT INTO queries (
    title,
    query,
    description
) VALUES ($1, $2, $3) RETURNING *;

-- name: get-query-by-uuid
SELECT * FROM queries WHERE uuid = $1;

-- name: get-query
SELECT * FROM queries WHERE id = $1;

-- name: get-queries
SELECT
    uuid,
    title,
    query,
    description,
    CEIL((SELECT COUNT(*) FROM queries)::float / $1) as page_count,
    (SELECT COUNT(*) FROM queries) as total_count
FROM queries
LIMIT $1 OFFSET $2;

-- name: delete-query-by-uuid
DELETE FROM queries WHERE uuid = $1;

-- name: update-query-by-uuid
UPDATE queries SET title = $1, query = $2, description = $3 WHERE uuid = $4 RETURNING uuid, query, description;

-- name: create-schedule
INSERT INTO schedules (
    query_id_fk,
    interval,
    platform,
    version,
    shard,
    denylist,
    removed,
    snapshot,
    title
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING *;

-- name: get-schedules
SELECT
    schedules.title,
    schedules.uuid,
    schedules.query_id_fk,
    schedules.interval,
    schedules.platform,
    schedules.version,
    schedules.shard,
    schedules.denylist,
    schedules.removed,
    schedules.snapshot,
    CEIL((SELECT COUNT(*) FROM schedules)::float / $1) as page_count,
    (SELECT COUNT(*) FROM schedules) as total_count
FROM schedules
LIMIT $1 OFFSET $2;
