CREATE TABLE IF NOT EXISTS {{ident .Database}}.{{ident .TableName}} {{.ClusterString}} (
    Timestamp DateTime64(9) CODEC(Delta(8), ZSTD(1)),
    TraceId String CODEC(ZSTD(1)),
    SpanId String CODEC(ZSTD(1)),
    ParentSpanId String CODEC(ZSTD(1)),
    TraceState String CODEC(ZSTD(1)),
    SpanName LowCardinality(String) CODEC(ZSTD(1)),
    SpanKind LowCardinality(String) CODEC(ZSTD(1)),
    ServiceName LowCardinality(String) CODEC(ZSTD(1)),
    ResourceAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    ScopeName String CODEC(ZSTD(1)),
    ScopeVersion String CODEC(ZSTD(1)),
    SpanAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    Duration UInt64 CODEC(T64, ZSTD(1)),
    StatusCode LowCardinality(String) CODEC(ZSTD(1)),
    StatusMessage String CODEC(ZSTD(1)),
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
    ResourceAttributeItems Array(String) ALIAS arrayMap(kv -> concat(kv.1, '=', kv.2), CAST(ResourceAttributes, 'Array(Tuple(String, String))')),
    SpanAttributeItems Array(String) ALIAS arrayMap(kv -> concat(kv.1, '=', kv.2), CAST(SpanAttributes, 'Array(Tuple(String, String))')),
{{- if .HasFullTextSearch}}
    INDEX idx_trace_id TraceId TYPE text(tokenizer = 'array'),
    INDEX idx_res_attr_key mapKeys(ResourceAttributes) TYPE text(tokenizer = 'array'),
    INDEX idx_res_attr_items ResourceAttributeItems TYPE text(tokenizer = 'array'),
    INDEX idx_span_attr_key mapKeys(SpanAttributes) TYPE text(tokenizer = 'array'),
    INDEX idx_span_attr_items SpanAttributeItems TYPE text(tokenizer = 'array'),
{{- else}}
    INDEX idx_trace_id TraceId TYPE bloom_filter(0.001) GRANULARITY 1,
    INDEX idx_res_attr_key mapKeys(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_res_attr_value mapValues(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_span_attr_key mapKeys(SpanAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_span_attr_value mapValues(SpanAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
{{- end}}
    INDEX idx_duration Duration TYPE minmax GRANULARITY 1
) ENGINE = {{.Engine}}
PARTITION BY toDate(Timestamp)
PRIMARY KEY (ServiceName, SpanName, toDateTime(Timestamp))
ORDER BY (ServiceName, SpanName, toDateTime(Timestamp), Timestamp)
{{.TTL}}
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1
