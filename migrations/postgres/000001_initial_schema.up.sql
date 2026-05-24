CREATE TABLE nodes (
    id BIGSERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuidv7(),
    node_key UUID NOT NULL DEFAULT uuidv7(),
    host_identifier TEXT NOT NULL,
    hostname TEXT NOT NULL DEFAULT '',
    platform TEXT NOT NULL DEFAULT '',
    os_name TEXT NOT NULL DEFAULT '',
    os_version TEXT NOT NULL DEFAULT '',
    osquery_version TEXT NOT NULL DEFAULT '',
    hardware_serial TEXT NOT NULL DEFAULT '',
    enrolled_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ,
    last_policy_check_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT nodes_uuid_unique UNIQUE (uuid),
    CONSTRAINT nodes_node_key_unique UNIQUE (node_key),
    CONSTRAINT nodes_host_identifier_unique UNIQUE (host_identifier)
);

CREATE INDEX nodes_last_seen_idx ON nodes (last_seen_at DESC);
CREATE INDEX nodes_platform_idx ON nodes (platform);
CREATE INDEX nodes_last_policy_check_at_idx ON nodes (last_policy_check_at);

CREATE TABLE queries (
    id BIGSERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuidv7(),
    name TEXT NOT NULL,
    sql TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    is_system BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT queries_uuid_unique UNIQUE (uuid),
    CONSTRAINT queries_name_unique UNIQUE (name),
    CONSTRAINT queries_name_nonempty CHECK (length(trim(name)) > 0),
    CONSTRAINT queries_sql_nonempty CHECK (length(trim(sql)) > 0)
);

CREATE INDEX queries_system_idx ON queries (is_system);
CREATE INDEX queries_created_at_idx ON queries (created_at DESC);

CREATE TABLE schedules (
    id BIGSERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuidv7(),
    query_id BIGINT NOT NULL REFERENCES queries (id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    interval_seconds INTEGER NOT NULL,
    platform TEXT NOT NULL DEFAULT 'all',
    version TEXT NOT NULL DEFAULT '',
    shard INTEGER NOT NULL DEFAULT 100,
    denylist BOOLEAN NOT NULL DEFAULT false,
    removed BOOLEAN NOT NULL DEFAULT false,
    snapshot BOOLEAN NOT NULL DEFAULT false,
    enabled BOOLEAN NOT NULL DEFAULT true,
    is_system BOOLEAN NOT NULL DEFAULT false,
    sql_version INTEGER NOT NULL DEFAULT 1,
    retention_days INTEGER NOT NULL DEFAULT 30,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT schedules_uuid_unique UNIQUE (uuid),
    CONSTRAINT schedules_name_unique UNIQUE (name),
    CONSTRAINT schedules_name_nonempty CHECK (length(trim(name)) > 0),
    CONSTRAINT schedules_interval_positive CHECK (interval_seconds > 0),
    CONSTRAINT schedules_interval_max CHECK (interval_seconds <= 604800),
    CONSTRAINT schedules_shard_range CHECK (shard >= 0 AND shard <= 100),
    CONSTRAINT schedules_platform_check CHECK (platform IN ('darwin', 'linux', 'posix', 'windows', 'any', 'all')),
    CONSTRAINT schedules_retention_days_range CHECK (retention_days BETWEEN 1 AND 365)
);

CREATE INDEX schedules_query_id_idx ON schedules (query_id);
CREATE INDEX schedules_enabled_idx ON schedules (enabled);
CREATE INDEX schedules_system_idx ON schedules (is_system);
CREATE INDEX schedules_created_at_idx ON schedules (created_at DESC);

CREATE TABLE osquery_status_logs (
    id BIGSERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuidv7(),
    node_id BIGINT NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    calendar_time TEXT NOT NULL DEFAULT '',
    file_name TEXT NOT NULL DEFAULT '',
    line INTEGER NOT NULL DEFAULT 0,
    message TEXT NOT NULL DEFAULT '',
    severity INTEGER NOT NULL DEFAULT 0,
    unix_time TIMESTAMPTZ,
    version TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT osquery_status_logs_uuid_unique UNIQUE (uuid)
);

CREATE INDEX osquery_status_logs_node_time_idx ON osquery_status_logs (node_id, unix_time DESC);
CREATE INDEX osquery_status_logs_severity_idx ON osquery_status_logs (severity);
CREATE INDEX osquery_status_logs_created_at_idx ON osquery_status_logs (created_at DESC);

CREATE TABLE policies (
    id BIGSERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuidv7(),
    name TEXT NOT NULL,
    query TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    resolution TEXT NOT NULL DEFAULT '',
    platform TEXT NOT NULL DEFAULT 'all',
    enabled BOOLEAN NOT NULL DEFAULT true,
    is_system BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT policies_uuid_unique UNIQUE (uuid),
    CONSTRAINT policies_name_unique UNIQUE (name),
    CONSTRAINT policies_name_nonempty CHECK (length(trim(name)) > 0),
    CONSTRAINT policies_query_nonempty CHECK (length(trim(query)) > 0),
    CONSTRAINT policies_platform_check CHECK (platform IN ('darwin', 'linux', 'posix', 'windows', 'any', 'all'))
);

CREATE INDEX policies_enabled_platform_idx ON policies (enabled, platform);
CREATE INDEX policies_created_at_idx ON policies (created_at DESC);

CREATE TABLE policy_membership (
    policy_id BIGINT NOT NULL REFERENCES policies (id) ON DELETE CASCADE,
    node_id BIGINT NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    passes BOOLEAN,
    last_error TEXT NOT NULL DEFAULT '',
    checked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (policy_id, node_id)
);

CREATE INDEX policy_membership_node_idx ON policy_membership (node_id);
CREATE INDEX policy_membership_policy_passes_idx ON policy_membership (policy_id, passes);
CREATE INDEX policy_membership_checked_at_idx ON policy_membership (checked_at DESC);

CREATE TABLE machine_query_results (
    id BIGSERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuidv7(),
    node_id BIGINT NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    query TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    results JSONB,
    error TEXT NOT NULL DEFAULT '',
    dispatched_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT machine_query_results_uuid_unique UNIQUE (uuid),
    CONSTRAINT machine_query_results_query_nonempty CHECK (length(trim(query)) > 0),
    CONSTRAINT machine_query_results_status_check CHECK (status IN ('pending', 'complete', 'error'))
);

CREATE INDEX machine_query_results_node_created_idx ON machine_query_results (node_id, created_at DESC);
CREATE INDEX machine_query_results_status_idx ON machine_query_results (status);
CREATE INDEX machine_query_results_pending_dispatch_idx ON machine_query_results (node_id, created_at)
    WHERE status = 'pending' AND dispatched_at IS NULL;

CREATE TABLE groups (
    id BIGSERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuidv7(),
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT groups_uuid_unique UNIQUE (uuid),
    CONSTRAINT groups_name_unique UNIQUE (name),
    CONSTRAINT groups_name_nonempty CHECK (length(trim(name)) > 0)
);

CREATE INDEX groups_created_at_idx ON groups (created_at DESC);

CREATE TABLE group_membership (
    group_id BIGINT NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
    node_id BIGINT NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (group_id, node_id)
);

CREATE INDEX group_membership_node_idx ON group_membership (node_id);
CREATE INDEX group_membership_group_idx ON group_membership (group_id);

CREATE TABLE policy_groups (
    policy_id BIGINT NOT NULL REFERENCES policies (id) ON DELETE CASCADE,
    group_id BIGINT NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (policy_id, group_id)
);

CREATE INDEX policy_groups_group_idx ON policy_groups (group_id);
CREATE INDEX policy_groups_policy_idx ON policy_groups (policy_id);

CREATE TABLE schedule_groups (
    schedule_id BIGINT NOT NULL REFERENCES schedules (id) ON DELETE CASCADE,
    group_id BIGINT NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (schedule_id, group_id)
);

CREATE INDEX schedule_groups_schedule_idx ON schedule_groups (schedule_id);
CREATE INDEX schedule_groups_group_idx ON schedule_groups (group_id);

CREATE TABLE query_schemas (
    schedule_uuid UUID NOT NULL,
    sql_version INTEGER NOT NULL,
    columns JSONB NOT NULL,
    first_observed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_observed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    row_count_estimate BIGINT NOT NULL DEFAULT 0,

    PRIMARY KEY (schedule_uuid, sql_version)
);

CREATE INDEX query_schemas_schedule_idx ON query_schemas (schedule_uuid);

INSERT INTO queries (name, sql, description, is_system)
VALUES
    (
        'Disk Usage POSIX',
        'SELECT path, type, ROUND((blocks_available * blocks_size * 10e-10), 2) AS free_gb, ROUND((blocks * blocks_size * 10e-10), 2) AS total_gb, ROUND ((blocks_available * 1.0 / blocks * 1.0) * 100, 2) AS free_perc FROM mounts WHERE path = ''/'';',
        'Retrieves disk usage information for the root mount point (POSIX)',
        true
    ),
    (
        'Disk Usage Windows',
        'SELECT device_id AS path, type, (free_space * 10e-10) AS free_gb, (size * 10e-10) AS total_gb, ROUND((free_space * 1.0 / size * 1.0) * 100, 2) AS free_perc FROM logical_drives WHERE device_id = ''C:'';',
        'Retrieves disk usage information for the C: drive (Windows)',
        true
    )
ON CONFLICT (name) DO UPDATE SET
    sql = EXCLUDED.sql,
    description = EXCLUDED.description,
    is_system = EXCLUDED.is_system,
    updated_at = now();

INSERT INTO schedules (name, query_id, interval_seconds, platform, snapshot, is_system)
SELECT 'Disk Monitoring POSIX', id, 3600, 'posix', true, true
FROM queries
WHERE name = 'Disk Usage POSIX'
ON CONFLICT (name) DO UPDATE SET
    query_id = EXCLUDED.query_id,
    interval_seconds = EXCLUDED.interval_seconds,
    platform = EXCLUDED.platform,
    snapshot = EXCLUDED.snapshot,
    is_system = EXCLUDED.is_system,
    updated_at = now();

INSERT INTO schedules (name, query_id, interval_seconds, platform, snapshot, is_system)
SELECT 'Disk Monitoring Windows', id, 3600, 'windows', true, true
FROM queries
WHERE name = 'Disk Usage Windows'
ON CONFLICT (name) DO UPDATE SET
    query_id = EXCLUDED.query_id,
    interval_seconds = EXCLUDED.interval_seconds,
    platform = EXCLUDED.platform,
    snapshot = EXCLUDED.snapshot,
    is_system = EXCLUDED.is_system,
    updated_at = now();
