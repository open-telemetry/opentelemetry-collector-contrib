CREATE TABLE IF NOT EXISTS %q.%q %s (
    Timestamp DateTime64(9) COMMENT 'Event timestamp with nanosecond precision' CODEC(Delta(8), ZSTD(1)),
    TimestampTime DateTime DEFAULT toDateTime(Timestamp) COMMENT 'Event timestamp truncated to seconds for partitioning',
    TraceId String COMMENT 'W3C trace identifier' CODEC(ZSTD(1)),
    SpanId String COMMENT 'W3C span identifier' CODEC(ZSTD(1)),
    TraceFlags UInt8 COMMENT 'W3C trace flags',
    SeverityText LowCardinality(String) COMMENT 'Log severity as text (e.g. INFO, ERROR)' CODEC(ZSTD(1)),
    SeverityNumber UInt8 COMMENT 'Log severity as number (1-24), maps to OTel severity',
    ServiceName LowCardinality(String) COMMENT 'Service that emitted the log (resource attribute service.name)' CODEC(ZSTD(1)),
    Body String COMMENT 'Log message body' CODEC(ZSTD(1)),
    ResourceSchemaUrl LowCardinality(String) COMMENT 'Schema URL for the resource' CODEC(ZSTD(1)),
    ResourceAttributes Map(LowCardinality(String), String) COMMENT 'Resource attributes as key-value pairs' CODEC(ZSTD(1)),
    ScopeSchemaUrl LowCardinality(String) COMMENT 'Schema URL for the instrumentation scope' CODEC(ZSTD(1)),
    ScopeName String COMMENT 'Instrumentation scope name (e.g. library name)' CODEC(ZSTD(1)),
    ScopeVersion LowCardinality(String) COMMENT 'Instrumentation scope version' CODEC(ZSTD(1)),
    ScopeAttributes Map(LowCardinality(String), String) COMMENT 'Instrumentation scope attributes' CODEC(ZSTD(1)),
    LogAttributes Map(LowCardinality(String), String) COMMENT 'Log record attributes as key-value pairs' CODEC(ZSTD(1)),
    EventName String COMMENT 'Event name for log records representing events' CODEC(ZSTD(1)),

    INDEX idx_trace_id TraceId TYPE bloom_filter(0.001) GRANULARITY 1,
    INDEX idx_res_attr_key mapKeys(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_res_attr_value mapValues(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_scope_attr_key mapKeys(ScopeAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_scope_attr_value mapValues(ScopeAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_log_attr_key mapKeys(LogAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_log_attr_value mapValues(LogAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_body Body TYPE tokenbf_v1(32768, 3, 0) GRANULARITY 8
) ENGINE = %s
PARTITION BY toDate(TimestampTime)
PRIMARY KEY (ServiceName, TimestampTime)
ORDER BY (ServiceName, TimestampTime, Timestamp)
%s
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1
