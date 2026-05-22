ALTER TABLE nodes ADD COLUMN policy_updated_at TIMESTAMPTZ;
CREATE INDEX nodes_policy_updated_at_idx ON nodes (policy_updated_at);

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
