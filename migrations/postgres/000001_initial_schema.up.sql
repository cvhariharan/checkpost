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

CREATE TABLE schedules (
    id BIGSERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuidv7(),
    name TEXT NOT NULL,
    sql TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
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
    CONSTRAINT schedules_sql_nonempty CHECK (length(trim(sql)) > 0),
    CONSTRAINT schedules_interval_positive CHECK (interval_seconds > 0),
    CONSTRAINT schedules_interval_max CHECK (interval_seconds <= 604800),
    CONSTRAINT schedules_shard_range CHECK (shard >= 0 AND shard <= 100),
    CONSTRAINT schedules_platform_check CHECK (platform IN ('darwin', 'linux', 'posix', 'windows', 'any', 'all')),
    CONSTRAINT schedules_retention_days_range CHECK (retention_days BETWEEN 1 AND 365)
);

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

CREATE TABLE yara_signature_sources (
    id BIGSERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuidv7(),
    group_id BIGINT REFERENCES groups (id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    label TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT yara_signature_sources_uuid_unique UNIQUE (uuid),
    CONSTRAINT yara_signature_sources_url_nonempty CHECK (length(trim(url)) > 0)
);

CREATE UNIQUE INDEX yara_signature_sources_default_url_unique_idx
    ON yara_signature_sources (lower(trim(url)))
    WHERE group_id IS NULL;

CREATE UNIQUE INDEX yara_signature_sources_group_url_unique_idx
    ON yara_signature_sources (group_id, lower(trim(url)))
    WHERE group_id IS NOT NULL;

CREATE INDEX yara_signature_sources_group_idx ON yara_signature_sources (group_id);
CREATE INDEX yara_signature_sources_enabled_idx ON yara_signature_sources (enabled);

CREATE TABLE yara_scans (
    id BIGSERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuidv7(),
    group_id BIGINT REFERENCES groups (id) ON DELETE SET NULL,
    paths TEXT[] NOT NULL,
    rule_urls TEXT[] NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    target_count INTEGER NOT NULL DEFAULT 0,
    completed_count INTEGER NOT NULL DEFAULT 0,
    match_count INTEGER NOT NULL DEFAULT 0,
    error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,

    CONSTRAINT yara_scans_uuid_unique UNIQUE (uuid),
    CONSTRAINT yara_scans_paths_nonempty CHECK (COALESCE(array_length(paths, 1), 0) > 0),
    CONSTRAINT yara_scans_rule_urls_nonempty CHECK (COALESCE(array_length(rule_urls, 1), 0) > 0),
    CONSTRAINT yara_scans_status_check CHECK (status IN ('pending', 'running', 'complete', 'partial', 'error'))
);

CREATE INDEX yara_scans_created_at_idx ON yara_scans (created_at DESC);
CREATE INDEX yara_scans_status_idx ON yara_scans (status);

CREATE TABLE yara_scan_targets (
    scan_id BIGINT NOT NULL REFERENCES yara_scans (id) ON DELETE CASCADE,
    node_id BIGINT NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending',
    dispatched_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (scan_id, node_id),
    CONSTRAINT yara_scan_targets_status_check CHECK (status IN ('pending', 'dispatched', 'complete', 'error'))
);

CREATE INDEX yara_scan_targets_node_status_created_idx ON yara_scan_targets (node_id, status, created_at);
CREATE INDEX yara_scan_targets_scan_status_idx ON yara_scan_targets (scan_id, status);

CREATE TABLE yara_scan_matches (
    id BIGSERIAL PRIMARY KEY,
    scan_id BIGINT NOT NULL REFERENCES yara_scans (id) ON DELETE CASCADE,
    node_id BIGINT NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    matches TEXT NOT NULL,
    count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT yara_scan_matches_count_positive CHECK (count > 0),
    CONSTRAINT yara_scan_matches_matches_nonempty CHECK (length(trim(matches)) > 0)
);

CREATE INDEX yara_scan_matches_scan_idx ON yara_scan_matches (scan_id);
CREATE INDEX yara_scan_matches_node_idx ON yara_scan_matches (node_id);
CREATE INDEX yara_scan_matches_created_at_idx ON yara_scan_matches (created_at DESC);

CREATE TABLE device_owners (
    id BIGSERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuidv7(),
    display_name TEXT NOT NULL,
    email TEXT NOT NULL DEFAULT '',
    external_id TEXT NOT NULL DEFAULT '',
    department TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    phone TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT device_owners_uuid_unique UNIQUE (uuid),
    CONSTRAINT device_owners_display_name_nonempty CHECK (length(trim(display_name)) > 0),
    CONSTRAINT device_owners_email_nonempty CHECK (length(trim(email)) > 0)
);

CREATE UNIQUE INDEX device_owners_email_unique_idx
    ON device_owners (lower(trim(email)))
    WHERE length(trim(email)) > 0;

CREATE INDEX device_owners_email_search_idx ON device_owners (lower(email));
CREATE UNIQUE INDEX device_owners_external_id_unique_idx
    ON device_owners (lower(trim(external_id)))
    WHERE length(trim(external_id)) > 0;

CREATE INDEX device_owners_display_name_idx ON device_owners (lower(display_name));
CREATE INDEX device_owners_created_at_idx ON device_owners (created_at DESC);

CREATE TABLE node_inventory (
    node_id BIGINT PRIMARY KEY REFERENCES nodes (id) ON DELETE CASCADE,
    owner_id BIGINT REFERENCES device_owners (id) ON DELETE SET NULL,
    internal_tracking_id TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX node_inventory_tracking_id_unique_idx
    ON node_inventory (lower(trim(internal_tracking_id)))
    WHERE length(trim(internal_tracking_id)) > 0;

CREATE INDEX node_inventory_owner_idx ON node_inventory (owner_id);
CREATE INDEX node_inventory_updated_at_idx ON node_inventory (updated_at DESC);

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

CREATE TABLE node_metrics (
    node_id BIGINT NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    value JSONB NOT NULL,
    collected_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (node_id, kind),
    CONSTRAINT node_metrics_kind_nonempty CHECK (length(trim(kind)) > 0)
);

CREATE INDEX node_metrics_kind_idx ON node_metrics (kind);
CREATE INDEX node_metrics_collected_at_idx ON node_metrics (collected_at DESC);

-- AUTHENTICATION & AUTHORIZATION ------------------------------------------

CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    uuid          UUID NOT NULL DEFAULT uuidv7(),
    username      TEXT NOT NULL,                 -- login id; for SSO users this is the email
    name          TEXT NOT NULL DEFAULT '',
    email         TEXT NOT NULL DEFAULT '',
    password_hash TEXT,                          -- bcrypt; NULL for SSO users
    login_type    TEXT NOT NULL DEFAULT 'oidc',  -- 'standard' | 'oidc'
    disabled      BOOLEAN NOT NULL DEFAULT false,
    last_login_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT users_uuid_unique UNIQUE (uuid),
    CONSTRAINT users_login_type_check CHECK (login_type IN ('standard','oidc'))
);
CREATE UNIQUE INDEX users_username_unique ON users (lower(username));

-- simplesessions postgres store (v3) backing table.
CREATE TABLE sessions (
    id         TEXT PRIMARY KEY,
    data       JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX sessions_created_at_idx ON sessions (created_at);

-- USER GROUPS (authz subjects) — distinct from machine `groups`.
CREATE TABLE user_groups (
    id               BIGSERIAL PRIMARY KEY,
    uuid             UUID NOT NULL DEFAULT uuidv7(),
    name             TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    oidc_claim_value TEXT NOT NULL DEFAULT '',  -- if set, OIDC `groups` claim entries matching this auto-populate membership
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT user_groups_uuid_unique UNIQUE (uuid),
    CONSTRAINT user_groups_name_unique UNIQUE (name)
);
CREATE UNIQUE INDEX user_groups_claim_idx ON user_groups (lower(oidc_claim_value))
    WHERE length(trim(oidc_claim_value)) > 0;

CREATE TABLE user_group_members (
    user_group_id BIGINT NOT NULL REFERENCES user_groups (id) ON DELETE CASCADE,
    user_id       BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    source        TEXT NOT NULL DEFAULT 'manual',  -- 'manual' | 'oidc'
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_group_id, user_id),
    CONSTRAINT user_group_members_source_check CHECK (source IN ('manual','oidc'))
);
CREATE INDEX user_group_members_user_idx ON user_group_members (user_id);

-- ROLE BINDINGS: subject (user XOR user_group) -> built-in role name, optionally
-- scoped to one machine group. Roles themselves are code-defined (no roles table).
CREATE TABLE role_bindings (
    id             BIGSERIAL PRIMARY KEY,
    uuid           UUID NOT NULL DEFAULT uuidv7(),
    user_id        BIGINT REFERENCES users (id) ON DELETE CASCADE,
    user_group_id  BIGINT REFERENCES user_groups (id) ON DELETE CASCADE,
    role           TEXT NOT NULL,                                    -- 'admin'|'operator'|'analyst'|'viewer'
    scope_group_id BIGINT REFERENCES groups (id) ON DELETE CASCADE,  -- NULL = global; FK -> machine groups
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT role_bindings_uuid_unique UNIQUE (uuid),
    CONSTRAINT role_bindings_role_check CHECK (role IN ('admin','operator','analyst','viewer')),
    CONSTRAINT role_bindings_single_subject CHECK (
        (user_id IS NOT NULL AND user_group_id IS NULL) OR
        (user_id IS NULL AND user_group_id IS NOT NULL)
    ),
    CONSTRAINT role_bindings_user_unique  UNIQUE (user_id, role, scope_group_id),
    CONSTRAINT role_bindings_group_unique UNIQUE (user_group_id, role, scope_group_id)
);
CREATE INDEX role_bindings_user_idx  ON role_bindings (user_id)       WHERE user_id IS NOT NULL;
CREATE INDEX role_bindings_group_idx ON role_bindings (user_group_id) WHERE user_group_id IS NOT NULL;

-- Casbin adapter table (memwey/casbin-sqlx-adapter). The adapter does NOT
-- auto-create this table — it validates existence and panics if missing — so
-- the migration MUST create it. Column names (`ptype`, `v0..v5`) match the
-- adapter's CasbinRule struct.
CREATE TABLE casbin_rule (
    id    BIGSERIAL PRIMARY KEY,
    ptype VARCHAR(100) NOT NULL DEFAULT '',
    v0    VARCHAR(256) NOT NULL DEFAULT '',
    v1    VARCHAR(256) NOT NULL DEFAULT '',
    v2    VARCHAR(256) NOT NULL DEFAULT '',
    v3    VARCHAR(256) NOT NULL DEFAULT '',
    v4    VARCHAR(256) NOT NULL DEFAULT '',
    v5    VARCHAR(256) NOT NULL DEFAULT '',
    CONSTRAINT casbin_rule_unique UNIQUE (ptype, v0, v1, v2, v3, v4, v5)
);

-- System schedules carry their SQL inline. The schedule name is the key the
-- systemmetrics registry maps to a metric kind.
INSERT INTO schedules (name, sql, description, interval_seconds, platform, snapshot, is_system)
VALUES
    (
        'Disk Usage POSIX',
        'SELECT path, type, ROUND((blocks_available * blocks_size * 10e-10), 2) AS free_gb, ROUND((blocks * blocks_size * 10e-10), 2) AS total_gb, ROUND ((blocks_available * 1.0 / blocks * 1.0) * 100, 2) AS free_perc FROM mounts WHERE path = ''/'';',
        'Disk usage for the root mount (POSIX).',
        3600, 'posix', true, true
    ),
    (
        'Disk Usage Windows',
        'SELECT device_id AS path, type, (free_space * 10e-10) AS free_gb, (size * 10e-10) AS total_gb, ROUND((free_space * 1.0 / size * 1.0) * 100, 2) AS free_perc FROM logical_drives WHERE device_id = ''C:'';',
        'Disk usage for the C: drive (Windows).',
        3600, 'windows', true, true
    ),
    (
        'Network Interfaces POSIX',
        'SELECT ia.interface AS name, ia.address AS address, id.mac AS mac FROM interface_addresses ia LEFT JOIN interface_details id ON id.interface = ia.interface;',
        'Network interface addresses and MACs (POSIX).',
        3600, 'posix', true, true
    ),
    (
        'Network Interfaces Windows',
        'SELECT ia.interface AS name, ia.address AS address, id.mac AS mac FROM interface_addresses ia LEFT JOIN interface_details id ON id.interface = ia.interface;',
        'Network interface addresses and MACs (Windows).',
        3600, 'windows', true, true
    ),
    (
        'Memory Usage POSIX',
        'SELECT memory_total AS total_bytes, memory_available AS available_bytes FROM memory_info;',
        'Total and available physical memory (POSIX).',
        3600, 'posix', true, true
    ),
    (
        'Memory Usage Windows',
        'SELECT physical_memory AS total_bytes FROM system_info;',
        'Total physical memory (Windows). Available memory not reported.',
        3600, 'windows', true, true
    ),
    (
        'CPU Info',
        'SELECT cpu_brand AS model, cpu_physical_cores AS physical_cores, cpu_logical_cores AS logical_cores FROM system_info;',
        'Primary CPU model and core counts.',
        86400, 'all', true, true
    ),
    (
        'OS Info',
        'SELECT name, version, build, platform FROM os_version;',
        'Operating system name, version, and platform.',
        86400, 'all', true, true
    ),
    (
        'System Uptime',
        'SELECT total_seconds AS seconds FROM uptime;',
        'System uptime in seconds.',
        3600, 'all', true, true
    )
ON CONFLICT (name) DO UPDATE SET
    sql = EXCLUDED.sql,
    description = EXCLUDED.description,
    interval_seconds = EXCLUDED.interval_seconds,
    platform = EXCLUDED.platform,
    snapshot = EXCLUDED.snapshot,
    is_system = EXCLUDED.is_system,
    updated_at = now();
