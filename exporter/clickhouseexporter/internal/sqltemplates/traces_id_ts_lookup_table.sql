CREATE TABLE IF NOT EXISTS "%s"."%s_trace_id_ts" %s (
    TraceId String CODEC(ZSTD(1)),
    Start DateTime CODEC(Delta, ZSTD(1)),
    End DateTime CODEC(Delta, ZSTD(1)),
    INDEX idx_trace_id TraceId TYPE bloom_filter(0.01) GRANULARITY 1
) ENGINE = %s
    PARTITION BY toDate(Start)
    ORDER BY (TraceId, Start)
    %s
    SETTINGS index_granularity=8192, ttl_only_drop_parts = 1