CREATE TABLE IF NOT EXISTS osquery_status (
    action LowCardinality (String),
    calendar_time String,
    counter UInt64,
    epoch UInt64,
    host_identifier String,
    name String,
    numerics Boolean,
    unix_time DateTime64 (0) DEFAULT 0,
    columns_json JSON (message String)
) ENGINE = MergeTree ()
ORDER BY
    unix_time
PARTITION BY
    toYYYYMM (unix_time)
