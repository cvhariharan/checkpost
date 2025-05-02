CREATE TABLE IF NOT EXISTS osquery_results (
    action LowCardinality (String),
    calendar_time String,
    counter UInt64,
    epoch UInt64,
    host_identifier String,
    name String,
    numerics Boolean,
    unix_time DateTime64 (0) DEFAULT 0,
    columns JSON
) ENGINE = ReplacingMergeTree ()
ORDER BY
    (
        host_identifier,
        toStartOfHour (unix_time),
        name,
        unix_time
    )
PARTITION BY
    toYYYYMM (unix_time) TTL toDateTime (unix_time) + INTERVAL 6 MONTH DELETE
