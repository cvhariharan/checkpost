-- name: AddNode :one
INSERT INTO
    nodes (host_identifier, os_name)
VALUES
    ($1, $2) RETURNING *;

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
