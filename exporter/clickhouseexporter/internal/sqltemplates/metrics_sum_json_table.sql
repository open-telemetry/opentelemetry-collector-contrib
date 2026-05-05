CREATE TABLE IF NOT EXISTS "%s"."%s" %s (
    ResourceAttributes JSON CODEC(ZSTD(1)),
    ResourceAttributesKeys Array(LowCardinality(String)) CODEC(ZSTD(1)),
    ResourceSchemaUrl String CODEC(ZSTD(1)),
    ScopeName String CODEC(ZSTD(1)),
    ScopeVersion String CODEC(ZSTD(1)),
    ScopeAttributes JSON CODEC(ZSTD(1)),
    ScopeAttributesKeys Array(LowCardinality(String)) CODEC(ZSTD(1)),
    ScopeDroppedAttrCount UInt32 CODEC(ZSTD(1)),
    ScopeSchemaUrl String CODEC(ZSTD(1)),
    ServiceName LowCardinality(String) CODEC(ZSTD(1)),
    MetricName String CODEC(ZSTD(1)),
    MetricDescription String CODEC(ZSTD(1)),
    MetricUnit String CODEC(ZSTD(1)),
    Attributes JSON CODEC(ZSTD(1)),
    AttributesKeys Array(LowCardinality(String)) CODEC(ZSTD(1)),
    StartTimeUnix DateTime64(9) CODEC(Delta, ZSTD(1)),
    TimeUnix DateTime64(9) CODEC(Delta, ZSTD(1)),
    Value Float64 CODEC(ZSTD(1)),
    Flags UInt32 CODEC(ZSTD(1)),
    Exemplars Nested (
        FilteredAttributes JSON,
        TimeUnix DateTime64(9),
        Value Float64,
        SpanId String,
        TraceId String
    ) CODEC(ZSTD(1)),
    AggregationTemporality Int32 CODEC(ZSTD(1)),
    IsMonotonic Boolean CODEC(Delta, ZSTD(1)),
    INDEX idx_res_attr_keys ResourceAttributesKeys TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_scope_attr_keys ScopeAttributesKeys TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_attr_keys AttributesKeys TYPE bloom_filter(0.01) GRANULARITY 1
) ENGINE = %s
%s
PARTITION BY toDate(TimeUnix)
ORDER BY (ServiceName, MetricName, toUnixTimestamp64Nano(TimeUnix))
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1
