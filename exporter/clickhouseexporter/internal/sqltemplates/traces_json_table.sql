CREATE TABLE IF NOT EXISTS %q.%q %s (
    Timestamp DateTime64(9) CODEC(Delta, ZSTD(3)),
    TraceId String CODEC(ZSTD(3)),
    SpanId String CODEC(ZSTD(3)),
    ParentSpanId String CODEC(ZSTD(3)),
    TraceState String CODEC(ZSTD(3)),
    SpanName LowCardinality(String) CODEC(ZSTD(3)),
    SpanKind LowCardinality(String) CODEC(ZSTD(3)),
    ServiceName LowCardinality(String) CODEC(ZSTD(3)),
    ResourceAttributes JSON CODEC(ZSTD(3)),
    ResourceAttributesKeys Array(LowCardinality(String)) CODEC(ZSTD(3)),
    ScopeName String CODEC(ZSTD(3)),
    ScopeVersion String CODEC(ZSTD(3)),
    SpanAttributes JSON CODEC(ZSTD(3)),
    SpanAttributesKeys Array(LowCardinality(String)) CODEC(ZSTD(3)),
    Duration UInt64 CODEC(ZSTD(3)),
    StatusCode LowCardinality(String) CODEC(ZSTD(3)),
    StatusMessage String CODEC(ZSTD(3)),
    Events Nested (
        Timestamp DateTime64(9),
        Name LowCardinality(String),
        Attributes JSON
    ) CODEC(ZSTD(3)),
    Links Nested (
        TraceId String,
        SpanId String,
        TraceState String,
        Attributes JSON
    ) CODEC(ZSTD(3)),

    INDEX idx_res_attr_keys ResourceAttributesKeys TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_span_attr_keys SpanAttributesKeys TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_duration Duration TYPE minmax GRANULARITY 1
) ENGINE = %s
PARTITION BY toDate(Timestamp)
ORDER BY (ServiceName, SpanName, toDateTime(Timestamp), Timestamp)
%s
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1
