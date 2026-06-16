DROP INDEX IF EXISTS machine_query_results_run_idx;
ALTER TABLE machine_query_results DROP COLUMN IF EXISTS run_id;
DROP TABLE IF EXISTS query_runs;
