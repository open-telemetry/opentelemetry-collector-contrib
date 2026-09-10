CREATE TABLE IF NOT EXISTS {{ident .Database}}.{{ident .TableName}} {{.ClusterString}} (
    TraceId String CODEC(ZSTD(1)),
    Start SimpleAggregateFunction(min, DateTime) CODEC(ZSTD(1)),
    End SimpleAggregateFunction(max, DateTime) CODEC(ZSTD(1))
) ENGINE = {{.Engine}}
PARTITION BY toStartOfWeek(Start)
ORDER BY (TraceId)
{{.TTL}}
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1
