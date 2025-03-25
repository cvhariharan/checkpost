CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS nodes (
    id SERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuid_generate_v4 (),
    host_identifier TEXT,
    os_name TEXT,
    created_at TIMESTAMP
    WITH
        TIME ZONE NOT NULL DEFAULT NOW ()
);

CREATE UNIQUE INDEX idx_node_uuid ON nodes (uuid);

CREATE TABLE IF NOT EXISTS os_version_info (
    id SERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuid_generate_v4 (),
    os_id TEXT,
    codename TEXT,
    major TEXT,
    minor TEXT,
    name TEXT,
    patch TEXT,
    platform TEXT,
    platform_like TEXT,
    version TEXT,
    node_fk INT NOT NULL,
    FOREIGN KEY (node_fk) REFERENCES nodes (id)
);

CREATE UNIQUE INDEX idx_os_version_info_uuid ON os_version_info (uuid);

CREATE TABLE IF NOT EXISTS osquery_info (
    id SERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuid_generate_v4 (),
    build_distro TEXT,
    build_platform TEXT,
    config_hash TEXT,
    config_valid TEXT,
    extensions TEXT,
    instance_id TEXT,
    pid TEXT,
    start_time TEXT,
    version TEXT,
    watcher TEXT,
    node_fk INT NOT NULL,
    FOREIGN KEY (node_fk) REFERENCES nodes (id)
);

CREATE UNIQUE INDEX idx_osquery_info_uuid ON osquery_info (uuid);

CREATE TABLE IF NOT EXISTS system_info (
    id SERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuid_generate_v4 (),
    computer_name TEXT,
    cpu_brand TEXT,
    cpu_logical_cores TEXT,
    cpu_physical_cores TEXT,
    cpu_subtype TEXT,
    cpu_type TEXT,
    hardware_model TEXT,
    hardware_serial TEXT,
    hardware_vendor TEXT,
    hardware_version TEXT,
    hostname TEXT,
    local_hostname TEXT,
    physical_memory TEXT,
    node_fk INT NOT NULL,
    FOREIGN KEY (node_fk) REFERENCES nodes (id)
);

CREATE UNIQUE INDEX idx_system_info_uuid ON system_info (uuid);

CREATE TABLE IF NOT EXISTS platform_info (
    id SERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuid_generate_v4 (),
    address TEXT,
    date TEXT,
    extra TEXT,
    revision TEXT,
    size TEXT,
    vendor TEXT,
    version TEXT,
    volume_size TEXT,
    node_fk INT NOT NULL,
    FOREIGN KEY (node_fk) REFERENCES nodes (id)
);

CREATE UNIQUE INDEX idx_platform_info_uuid ON platform_info (uuid);
