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
