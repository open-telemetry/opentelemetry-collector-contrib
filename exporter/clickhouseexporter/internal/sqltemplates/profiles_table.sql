CREATE TABLE IF NOT EXISTS %q.%q %s (
    Timestamp DateTime64(9) CODEC(Delta, ZSTD(1)),
    ProfileId String CODEC(ZSTD(1)),
    SampleType LowCardinality(String) CODEC(ZSTD(1)),
    SampleUnit LowCardinality(String) CODEC(ZSTD(1)),
    ServiceName LowCardinality(String) CODEC(ZSTD(1)),
    ResourceAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    ScopeName String CODEC(ZSTD(1)),
    ScopeVersion String CODEC(ZSTD(1)),
    ProfileAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    SampleAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    StackHash UInt64 CODEC(ZSTD(1)),
    Addresses Array(UInt64) CODEC(ZSTD(1)),
    FunctionNames Array(String) CODEC(ZSTD(1)),
    FileNames Array(String) CODEC(ZSTD(1)),
    LineNumbers Array(Int32) CODEC(ZSTD(1)),
    MappingFileNames Array(String) CODEC(ZSTD(1)),
    Values Array(Int64) CODEC(ZSTD(1)),
    TimestampsUnixNano Array(UInt64) CODEC(ZSTD(1)),
    DurationNano UInt64 CODEC(ZSTD(1)),
    Period Int64 CODEC(ZSTD(1)),
    PeriodType LowCardinality(String) CODEC(ZSTD(1)),
    PeriodUnit LowCardinality(String) CODEC(ZSTD(1)),
    TraceId String CODEC(ZSTD(1)),
    SpanId String CODEC(ZSTD(1)),
    INDEX idx_profile_id ProfileId TYPE bloom_filter(0.001) GRANULARITY 1,
    INDEX idx_trace_id TraceId TYPE bloom_filter(0.001) GRANULARITY 1,
    INDEX idx_res_attr_key mapKeys(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_res_attr_value mapValues(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_sample_attr_key mapKeys(SampleAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_sample_attr_value mapValues(SampleAttributes) TYPE bloom_filter(0.01) GRANULARITY 1
) ENGINE = %s
PARTITION BY toDate(Timestamp)
ORDER BY (ServiceName, SampleType, toDateTime(Timestamp))
%s
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1
