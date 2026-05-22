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
