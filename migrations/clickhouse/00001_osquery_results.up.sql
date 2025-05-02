CREATE TABLE IF NOT EXISTS osquery_results (
    action LowCardinality (String),
    calendar_time String,
    counter UInt64,
    epoch UInt64,
    host_identifier String,
    name LowCardinality (String),
    numerics Boolean,
    unix_time DateTime64 (0) DEFAULT 0,
    columns JSON CODEC (ZSTD (3)),
    columns_hash UInt64 MATERIALIZED cityHash64 (toString (columns))
) ENGINE = ReplacingMergeTree ()
ORDER BY
    (
        host_identifier,
        name,
        toStartOfHour (unix_time),
        columns_hash
    )
PARTITION BY
    toYYYYMM (unix_time) TTL toDateTime (unix_time) + INTERVAL 6 MONTH DELETE
