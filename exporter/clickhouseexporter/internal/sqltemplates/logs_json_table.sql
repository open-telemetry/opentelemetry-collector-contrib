CREATE TABLE IF NOT EXISTS {{ident .Database}}.{{ident .TableName}} {{.ClusterString}} (
    `Timestamp` DateTime64(9) CODEC(Delta(8), ZSTD(3)),
    `TraceId` String CODEC(ZSTD(3)),
    `SpanId` String CODEC(ZSTD(3)),
    `TraceFlags` UInt8,
    `SeverityText` LowCardinality(String) CODEC(ZSTD(3)),
    `SeverityNumber` UInt8,
    `ServiceName` LowCardinality(String) CODEC(ZSTD(3)),
    `Body` String CODEC(ZSTD(3)),
    `ResourceSchemaUrl` LowCardinality(String) CODEC(ZSTD(3)),
    `ResourceAttributes` JSON CODEC(ZSTD(3)),
    `ResourceAttributesKeys` Array(LowCardinality(String)) CODEC(ZSTD(3)),
    `ScopeSchemaUrl` LowCardinality(String) CODEC(ZSTD(3)),
    `ScopeName` String CODEC(ZSTD(3)),
    `ScopeVersion` LowCardinality(String) CODEC(ZSTD(3)),
    `ScopeAttributes` JSON CODEC(ZSTD(3)),
    `ScopeAttributesKeys` Array(LowCardinality(String)) CODEC(ZSTD(3)),
    `LogAttributes` JSON CODEC(ZSTD(3)),
    `LogAttributesKeys` Array(LowCardinality(String)) CODEC(ZSTD(3)),
    `EventName` String CODEC(ZSTD(3)),

    INDEX idx_res_attr_keys ResourceAttributesKeys TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_scope_attr_keys ScopeAttributesKeys TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_log_attr_keys LogAttributesKeys TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_body Body TYPE tokenbf_v1(32768, 3, 0) GRANULARITY 8
) ENGINE = {{.Engine}}
PARTITION BY toDate(Timestamp)
PRIMARY KEY (toStartOfFiveMinutes(Timestamp), ServiceName)
ORDER BY (toStartOfFiveMinutes(Timestamp), ServiceName, Timestamp)
{{.TTL}}
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1
