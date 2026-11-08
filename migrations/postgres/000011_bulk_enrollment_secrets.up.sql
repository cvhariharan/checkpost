ALTER TABLE enrollment_secrets ADD COLUMN name TEXT;

-- Drop anonymous enrollment entirely; mass enrollment now uses bulk secrets.
DROP INDEX IF EXISTS enrollment_secrets_one_active_anon_idx;
DELETE FROM enrollment_secrets WHERE type = 'anonymous';

ALTER TABLE enrollment_secrets DROP CONSTRAINT enrollment_secrets_type_check;
ALTER TABLE enrollment_secrets ADD CONSTRAINT enrollment_secrets_type_check
    CHECK (type IN ('owned', 'bulk'));

CREATE UNIQUE INDEX enrollment_secrets_active_bulk_name_idx
    ON enrollment_secrets (name) WHERE type = 'bulk' AND revoked_at IS NULL;
