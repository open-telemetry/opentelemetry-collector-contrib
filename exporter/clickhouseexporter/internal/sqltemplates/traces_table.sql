CREATE TABLE IF NOT EXISTS %q.%q %s (
    Timestamp DateTime64(9) CODEC(Delta, ZSTD(3)),
    TraceId String CODEC(ZSTD(3)),
    SpanId String CODEC(ZSTD(3)),
    ParentSpanId String CODEC(ZSTD(3)),
    TraceState String CODEC(ZSTD(3)),
    SpanName LowCardinality(String) CODEC(ZSTD(3)),
    SpanKind LowCardinality(String) CODEC(ZSTD(3)),
    ServiceName LowCardinality(String) CODEC(ZSTD(3)),
    ResourceAttributes Map(LowCardinality(String), String) CODEC(ZSTD(3)),
    ScopeName String CODEC(ZSTD(3)),
    ScopeVersion String CODEC(ZSTD(3)),
    SpanAttributes Map(LowCardinality(String), String) CODEC(ZSTD(3)),
    Duration UInt64 CODEC(ZSTD(3)),
    StatusCode LowCardinality(String) CODEC(ZSTD(3)),
    StatusMessage String CODEC(ZSTD(3)),
    Events Nested (
        Timestamp DateTime64(9),
        Name LowCardinality(String),
        Attributes Map(LowCardinality(String), String)
    ) CODEC(ZSTD(3)),
    Links Nested (
        TraceId String,
        SpanId String,
        TraceState String,
        Attributes Map(LowCardinality(String), String)
    ) CODEC(ZSTD(3)),
    INDEX idx_trace_id TraceId TYPE bloom_filter(0.001) GRANULARITY 1,
    INDEX idx_res_attr_key mapKeys(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_res_attr_value mapValues(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_span_attr_key mapKeys(SpanAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_span_attr_value mapValues(SpanAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_duration Duration TYPE minmax GRANULARITY 1
) ENGINE = %s
PARTITION BY toDate(Timestamp)
ORDER BY (ServiceName, SpanName, toDateTime(Timestamp))
%s
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1
