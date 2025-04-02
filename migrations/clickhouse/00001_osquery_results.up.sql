CREATE TABLE IF NOT EXISTS osquery_results (
    action LowCardinality (String),
    calendar_time String,
    counter UInt64,
    epoch UInt64,
    host_identifier String,
    name String,
    numerics Boolean,
    unix_time DateTime64 (0) DEFAULT 0,
    columns_json JSON
) ENGINE = MergeTree ()
ORDER BY
    unix_time
PARTITION BY
    toYYYYMM (unix_time)
