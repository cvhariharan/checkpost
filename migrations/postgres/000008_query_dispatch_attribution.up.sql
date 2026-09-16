ALTER TABLE machine_query_results
    ADD COLUMN created_by BIGINT REFERENCES users(id) ON DELETE SET NULL;

UPDATE machine_query_results AS result
SET created_by = run.created_by
FROM query_runs AS run
WHERE result.run_id = run.id;

CREATE INDEX machine_query_results_created_by_idx ON machine_query_results (created_by);
