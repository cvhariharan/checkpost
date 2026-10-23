ALTER TABLE users
    ADD COLUMN type TEXT NOT NULL DEFAULT 'user' CHECK (type IN ('user', 'service'));
