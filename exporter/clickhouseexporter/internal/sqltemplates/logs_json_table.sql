CREATE TABLE IF NOT EXISTS %q.%q %s (
    Timestamp DateTime64(9) CODEC(Delta(8), ZSTD(1)),
    TraceId String CODEC(ZSTD(1)),
    SpanId String CODEC(ZSTD(1)),
    TraceFlags UInt8,
    SeverityText LowCardinality(String) CODEC(ZSTD(1)),
    SeverityNumber UInt8,
    ServiceName LowCardinality(String) CODEC(ZSTD(1)),
    Body String CODEC(ZSTD(1)),
    ResourceSchemaUrl LowCardinality(String) CODEC(ZSTD(1)),
    ResourceAttributes JSON CODEC(ZSTD(1)),
    ResourceAttributesKeys Array(LowCardinality(String)) CODEC(ZSTD(1)),
    ScopeSchemaUrl LowCardinality(String) CODEC(ZSTD(1)),
    ScopeName String CODEC(ZSTD(1)),
    ScopeVersion LowCardinality(String) CODEC(ZSTD(1)),
    ScopeAttributes JSON CODEC(ZSTD(1)),
    ScopeAttributesKeys Array(LowCardinality(String)) CODEC(ZSTD(1)),
    LogAttributes JSON CODEC(ZSTD(1)),
    LogAttributesKeys Array(LowCardinality(String)) CODEC(ZSTD(1)),
    EventName String CODEC(ZSTD(1)),

    INDEX idx_res_attr_keys ResourceAttributesKeys TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_scope_attr_keys ScopeAttributesKeys TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_log_attr_keys LogAttributesKeys TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_body Body TYPE tokenbf_v1(32768, 3, 0) GRANULARITY 8
) ENGINE = %s
PARTITION BY toDate(Timestamp)
PRIMARY KEY (ServiceName, toDateTime(Timestamp))
ORDER BY (ServiceName, toDateTime(Timestamp), Timestamp)
%s
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1
