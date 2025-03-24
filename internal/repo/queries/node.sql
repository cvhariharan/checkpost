-- name: AddNode :one
INSERT INTO
    nodes (host_identifier, os_name)
VALUES
    ($1, $2) RETURNING *;

-- name: GetNodeByUUID :one
WITH node AS (
    SELECT
        *
    FROM
        nodes
    WHERE
        nodes.uuid = $1
) SELECT
    node.*,
    os_version_info.os_id,
    os_version_info.codename,
    os_version_info.major,
    os_version_info.minor,
    os_version_info.platform,
    os_version_info.platform_like,
    os_version_info.version,
    system_info.computer_name,
    system_info.cpu_logical_cores,
    system_info.cpu_physical_cores,
    system_info.hostname,
    system_info.local_hostname,
    system_info.physical_memory
    FROM
        node, os_version_info, osquery_info, system_info
    WHERE
        node.id = os_version_info.node_fk
        AND node.id = osquery_info.node_fk
        AND node.id = system_info.node_fk;

-- name: AddOSVersionInfo :one
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

-- name: GetOSVersionInfoByNode :one
SELECT * FROM os_version_info WHERE node_fk = $1;

-- name: AddOSQueryInfo :one
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

-- name: GetOSQueryInfoByNode :one
SELECT * FROM osquery_info WHERE node_fk = $1;

-- name: AddSystemInfo :one
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

-- name: GetSystemInfoByNode :one
SELECT * FROM system_info WHERE node_fk = $1;

-- name: AddPlatformInfo :one
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

-- name: GetPlatformInfoByNode :one
SELECT * FROM platform_info WHERE node_fk = $1;
