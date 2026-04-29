CREATE TABLE IF NOT EXISTS %q.%q %s (
    Timestamp DateTime64(9) COMMENT 'Span start timestamp with nanosecond precision' CODEC(Delta, ZSTD(1)),
    TraceId String COMMENT 'W3C trace identifier' CODEC(ZSTD(1)),
    SpanId String COMMENT 'Unique span identifier within the trace' CODEC(ZSTD(1)),
    ParentSpanId String COMMENT 'Span identifier of the parent span' CODEC(ZSTD(1)),
    TraceState String COMMENT 'W3C tracestate header value' CODEC(ZSTD(1)),
    SpanName LowCardinality(String) COMMENT 'Logical name of the span operation' CODEC(ZSTD(1)),
    SpanKind LowCardinality(String) COMMENT 'Span kind (CLIENT, SERVER, PRODUCER, CONSUMER, INTERNAL)' CODEC(ZSTD(1)),
    ServiceName LowCardinality(String) COMMENT 'Service that emitted the span (resource attribute service.name)' CODEC(ZSTD(1)),
    ResourceAttributes Map(LowCardinality(String), String) COMMENT 'Resource attributes as key-value pairs' CODEC(ZSTD(1)),
    ScopeName String COMMENT 'Instrumentation scope name (e.g. library name)' CODEC(ZSTD(1)),
    ScopeVersion String COMMENT 'Instrumentation scope version' CODEC(ZSTD(1)),
    SpanAttributes Map(LowCardinality(String), String) COMMENT 'Span attributes as key-value pairs' CODEC(ZSTD(1)),
    Duration UInt64 COMMENT 'Span duration in nanoseconds' CODEC(ZSTD(1)),
    StatusCode LowCardinality(String) COMMENT 'Span status code (UNSET, OK, ERROR)' CODEC(ZSTD(1)),
    StatusMessage String COMMENT 'Human-readable status message for errors' CODEC(ZSTD(1)),
    Events Nested (
        Timestamp DateTime64(9),
        Name LowCardinality(String),
        Attributes Map(LowCardinality(String), String)
    ) CODEC(ZSTD(1)),
    Links Nested (
        TraceId String,
        SpanId String,
        TraceState String,
        Attributes Map(LowCardinality(String), String)
    ) CODEC(ZSTD(1)),
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
