CREATE TABLE saved_queries (
    id BIGSERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuidv7(),
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    query TEXT NOT NULL,
    targets JSONB NOT NULL DEFAULT '{}'::jsonb,
    visibility TEXT NOT NULL DEFAULT 'private',
    created_by BIGINT REFERENCES users (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT saved_queries_uuid_unique UNIQUE (uuid),
    CONSTRAINT saved_queries_name_nonempty CHECK (length(trim(name)) > 0),
    CONSTRAINT saved_queries_query_nonempty CHECK (length(trim(query)) > 0),
    CONSTRAINT saved_queries_visibility_check CHECK (visibility IN ('private', 'public'))
);

CREATE UNIQUE INDEX saved_queries_owner_name_lower_idx ON saved_queries (created_by, lower(name));
CREATE INDEX saved_queries_created_idx ON saved_queries (created_at DESC);
