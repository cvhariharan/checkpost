CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE disk_usage (
    host_identifier TEXT NOT NULL,
    path TEXT NOT NULL,
    type TEXT NOT NULL,
    total_gb DECIMAL,
    free_gb DECIMAL,
    CONSTRAINT fk_host_identifier FOREIGN KEY (host_identifier) REFERENCES nodes (host_identifier)
);

CREATE UNIQUE INDEX disk_usage_host_identifier_path_idx ON disk_usage (host_identifier, path);
