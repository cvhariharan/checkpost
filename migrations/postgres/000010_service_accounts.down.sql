DELETE FROM users WHERE type = 'service';
ALTER TABLE users DROP COLUMN type;
