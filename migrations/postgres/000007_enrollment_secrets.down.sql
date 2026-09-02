DROP INDEX IF EXISTS nodes_enrollment_secret_idx;
ALTER TABLE nodes DROP COLUMN IF EXISTS enrollment_secret_id;

DROP TABLE IF EXISTS enrollment_secrets;
