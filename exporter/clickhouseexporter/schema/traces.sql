-- Required for `JSON` data type
allow_experimental_object_type = 1

CREATE TABLE IF NOT EXISTS canva.spans (
    Timestamp DateTime64(9) CODEC(Delta, ZSTD(1)),
    TraceId String CODEC(ZSTD(1)),
    SpanId String CODEC(ZSTD(1)),
    ParentSpanId String CODEC(ZSTD(1)),
    -- So long as we have less then 100,000 distinct span names, LowCardinality(String)
    -- should outperform String.
    -- See: https://clickhouse.com/docs/en/sql-reference/data-types/lowcardinality
    SpanName LowCardinality(String) CODEC(ZSTD(1)),
    SpanKind LowCardinality(String) CODEC(ZSTD(1)),
    ServiceName LowCardinality(String) CODEC(ZSTD(1)),
    ResourceAttributes JSON CODEC(ZSTD(1)),
    ScopeName String CODEC(ZSTD(1)),
    ScopeVersion String CODEC(ZSTD(1)),
    SpanAttributes JSON CODEC(ZSTD(1)),
    Duration Int64 CODEC(ZSTD(1)),
    StatusCode LowCardinality(String) CODEC(ZSTD(1)),
    StatusMessage String CODEC(ZSTD(1)),
    Events Nested (
        Timestamp DateTime64(9),
        Name LowCardinality(String),
        Attributes JSON
    ) CODEC(ZSTD(1)),
    Links Nested (
        TraceId String,
        SpanId String,
        Attributes JSON
    ) CODEC(ZSTD(1)),
    INDEX idx_trace_id TraceId TYPE bloom_filter(0.001) GRANULARITY 1,
    -- INDEX idx_res_attr_key mapKeys(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    -- INDEX idx_res_attr_value mapValues(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    -- INDEX idx_span_attr_key mapKeys(SpanAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    -- INDEX idx_span_attr_value mapValues(SpanAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
) ENGINE MergeTree()
-- Consider amending this TTL
TTL timestamp + INTERVAL 12 HOUR
-- TODO: I don't think partitioning by time will scale for us.
-- Maybe this should be (ServiceName, SpanName, toHour(Timestamp))
PARTITION BY toDate(Timestamp)
ORDER BY (ServiceName, SpanName, toUnixTimestamp(Timestamp), TraceId)
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1;



-- Notable changes from existing schema
-- 
-- 1. Removed [TraceState](https://opentelemetry.io/docs/specs/otel/trace/api/#tracestate) from
--    root column and `Links` data structure because this isn't used by our tracing API. This
--    would be an empty string for all cases. If we find a need for it, we can add it later.
--
-- 2. Attributes do not use a `Map(LowCardinality(String), String)` and instead use the experimental
--    `JSON` data type. We prefer using `JSON` so that original types are preserved (e.g. an int is
--    not cast to a string)JSON
--
-- 3. Partitioning by Date is not scaleable. 
--
-- 4. Removed `INDEX idx_duration Duration TYPE minmax GRANULARITY 1` - not sure why we need to have
--    an index of minimum and maximum duration?

-- 5. Removed various indexes on attribute key and value columns
