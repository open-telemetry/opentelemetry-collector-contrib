CREATE TABLE IF NOT EXISTS {{ident .Database}}.{{ident .TableName}} {{.ClusterString}} (
    TraceId String CODEC(ZSTD(3)),
{{- if .AggregatingEngine}}
    Start SimpleAggregateFunction(min, DateTime) CODEC(ZSTD(3)),
    End SimpleAggregateFunction(max, DateTime) CODEC(ZSTD(3))
{{- else}}
    Start DateTime CODEC(ZSTD(3)),
    End DateTime CODEC(ZSTD(3))
{{- end}}
) ENGINE = {{.Engine}}
PARTITION BY toStartOfWeek(Start)
{{- if .AggregatingEngine}}
ORDER BY (TraceId)
{{- else}}
ORDER BY (TraceId, Start)
{{- end}}
{{.TTL}}
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1
