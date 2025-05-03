CREATE TABLE IF NOT EXISTS osquery_status (
    calendar_time String,
    file_name LowCardinality (String),
    host_identifier String,
    Line UInt32,
    message String,
    unix_time DateTime64 (0) DEFAULT 0,
    Severity UInt8,
    Version LowCardinality (String),
    message_hash UInt64 MATERIALIZED cityHash64 (message)
) ENGINE = ReplacingMergeTree ()
ORDER BY
    (
        host_identifier,
        file_name,
        toStartOfHour (unix_time),
        message_hash
    )
PARTITION BY
    toYYYYMM (unix_time) TTL toDateTime (unix_time) + INTERVAL 6 MONTH DELETE
