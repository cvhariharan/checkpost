-- Multi-host query: a query_run groups one SQL submission's per-host ad-hoc
-- executions. Each targeted host still gets its own machine_query_results row
-- (keyed by its own uuid) so the existing distributed read/write path is
-- unchanged; run_id ties those rows to the run.

CREATE TABLE query_runs (
    id BIGSERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuidv7(),
    query TEXT NOT NULL,
    targets JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by BIGINT REFERENCES users (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT query_runs_uuid_unique UNIQUE (uuid),
    CONSTRAINT query_runs_query_nonempty CHECK (length(trim(query)) > 0)
);

CREATE INDEX query_runs_created_idx ON query_runs (created_at DESC);

-- Link per-host ad-hoc executions to their run. Nullable: single-host ad-hoc
-- queries (run from the machine detail page) keep run_id NULL and are unaffected.
ALTER TABLE machine_query_results
    ADD COLUMN run_id BIGINT REFERENCES query_runs (id) ON DELETE CASCADE;

CREATE INDEX machine_query_results_run_idx ON machine_query_results (run_id);
