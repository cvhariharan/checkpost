CREATE TABLE enrollment_secrets (
    id              BIGSERIAL PRIMARY KEY,
    uuid            UUID NOT NULL DEFAULT uuidv7(),
    secret_id       BYTEA NOT NULL,              -- 16-byte nonce; the public Secret ID
    type            TEXT NOT NULL,               -- 'anonymous' | 'owned'
    owner_user_uuid UUID,                        -- embedded owner for 'owned' (NULL for anonymous)
    created_by      BIGINT REFERENCES users (id) ON DELETE SET NULL,  -- who generated (NULL = system/anonymous)
    revoked_at      TIMESTAMPTZ,                 -- NULL = active
    revoked_by      BIGINT REFERENCES users (id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT enrollment_secrets_uuid_unique      UNIQUE (uuid),
    CONSTRAINT enrollment_secrets_secret_id_unique UNIQUE (secret_id),
    CONSTRAINT enrollment_secrets_type_check       CHECK (type IN ('anonymous','owned'))
);

CREATE UNIQUE INDEX enrollment_secrets_one_active_anon_idx
    ON enrollment_secrets ((true)) WHERE type = 'anonymous' AND revoked_at IS NULL;
CREATE UNIQUE INDEX enrollment_secrets_active_owned_idx
    ON enrollment_secrets (owner_user_uuid) WHERE type = 'owned' AND revoked_at IS NULL;

ALTER TABLE nodes ADD COLUMN enrollment_secret_id BIGINT
    REFERENCES enrollment_secrets (id) ON DELETE SET NULL;
CREATE INDEX nodes_enrollment_secret_idx ON nodes (enrollment_secret_id);
