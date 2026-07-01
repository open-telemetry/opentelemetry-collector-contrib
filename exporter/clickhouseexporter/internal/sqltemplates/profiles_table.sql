CREATE TABLE IF NOT EXISTS %q.%q %s (
    Timestamp DateTime64(9) CODEC(Delta, Default),
    ProfileId String,
    SampleType LowCardinality(String),
    SampleUnit LowCardinality(String),
    ServiceName LowCardinality(String),
    ResourceAttributes Map(LowCardinality(String), String),
    ScopeName String,
    ScopeVersion String,
    ProfileAttributes Map(LowCardinality(String), String),
    SampleAttributes Map(LowCardinality(String), String),
    StackHash UInt64,
    Addresses Array(UInt64),
    FunctionNames Array(String),
    FileNames Array(String),
    LineNumbers Array(Int32),
    MappingFileNames Array(String),
    Values Array(Int64),
    TimestampsUnixNano Array(UInt64),
    DurationNano UInt64,
    Period Int64,
    PeriodType LowCardinality(String),
    PeriodUnit LowCardinality(String),
    TraceId String,
    SpanId String,
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
