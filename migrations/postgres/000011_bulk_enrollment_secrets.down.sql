DROP INDEX IF EXISTS enrollment_secrets_active_bulk_name_idx;

DELETE FROM enrollment_secrets WHERE type = 'bulk';

ALTER TABLE enrollment_secrets DROP CONSTRAINT enrollment_secrets_type_check;
ALTER TABLE enrollment_secrets ADD CONSTRAINT enrollment_secrets_type_check
    CHECK (type IN ('anonymous', 'owned'));

CREATE UNIQUE INDEX enrollment_secrets_one_active_anon_idx
    ON enrollment_secrets ((true)) WHERE type = 'anonymous' AND revoked_at IS NULL;

ALTER TABLE enrollment_secrets DROP COLUMN name;
