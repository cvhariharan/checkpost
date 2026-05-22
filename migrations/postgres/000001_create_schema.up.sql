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
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT nodes_uuid_unique UNIQUE (uuid),
    CONSTRAINT nodes_node_key_unique UNIQUE (node_key),
    CONSTRAINT nodes_host_identifier_unique UNIQUE (host_identifier)
);

CREATE INDEX nodes_last_seen_idx ON nodes (last_seen_at DESC);
CREATE INDEX nodes_platform_idx ON nodes (platform);

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
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT schedules_uuid_unique UNIQUE (uuid),
    CONSTRAINT schedules_name_unique UNIQUE (name),
    CONSTRAINT schedules_name_nonempty CHECK (length(trim(name)) > 0),
    CONSTRAINT schedules_interval_positive CHECK (interval_seconds > 0),
    CONSTRAINT schedules_interval_max CHECK (interval_seconds <= 604800),
    CONSTRAINT schedules_shard_range CHECK (shard >= 0 AND shard <= 100),
    CONSTRAINT schedules_platform_check CHECK (platform IN ('darwin', 'linux', 'posix', 'windows', 'any', 'all'))
);

CREATE INDEX schedules_query_id_idx ON schedules (query_id);
CREATE INDEX schedules_enabled_idx ON schedules (enabled);
CREATE INDEX schedules_system_idx ON schedules (is_system);
CREATE INDEX schedules_created_at_idx ON schedules (created_at DESC);

CREATE TABLE osquery_result_batches (
    id BIGSERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuidv7(),
    node_id BIGINT NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    schedule_name TEXT NOT NULL,
    action TEXT NOT NULL,
    calendar_time TEXT NOT NULL DEFAULT '',
    counter BIGINT NOT NULL DEFAULT 0,
    epoch BIGINT NOT NULL DEFAULT 0,
    numerics BOOLEAN NOT NULL DEFAULT false,
    unix_time TIMESTAMPTZ,
    is_system BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT osquery_result_batches_uuid_unique UNIQUE (uuid),
    CONSTRAINT osquery_result_batches_action_check CHECK (action IN ('added', 'removed', 'snapshot'))
);

CREATE INDEX osquery_result_batches_node_time_idx ON osquery_result_batches (node_id, unix_time DESC);
CREATE INDEX osquery_result_batches_schedule_time_idx ON osquery_result_batches (schedule_name, unix_time DESC);
CREATE INDEX osquery_result_batches_system_idx ON osquery_result_batches (is_system);

CREATE TABLE osquery_result_rows (
    id BIGSERIAL PRIMARY KEY,
    batch_id BIGINT NOT NULL REFERENCES osquery_result_batches (id) ON DELETE CASCADE,
    row_index INTEGER NOT NULL,

    CONSTRAINT osquery_result_rows_unique UNIQUE (batch_id, row_index)
);

CREATE TABLE osquery_result_cells (
    id BIGSERIAL PRIMARY KEY,
    row_id BIGINT NOT NULL REFERENCES osquery_result_rows (id) ON DELETE CASCADE,
    column_name TEXT NOT NULL,
    value_text TEXT NOT NULL DEFAULT '',

    CONSTRAINT osquery_result_cells_unique UNIQUE (row_id, column_name),
    CONSTRAINT osquery_result_cells_column_nonempty CHECK (length(trim(column_name)) > 0)
);

CREATE INDEX osquery_result_cells_column_value_idx ON osquery_result_cells (column_name, value_text);
CREATE INDEX osquery_result_cells_row_id_idx ON osquery_result_cells (row_id);

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
