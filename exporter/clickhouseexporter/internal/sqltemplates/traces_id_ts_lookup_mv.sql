CREATE MATERIALIZED VIEW IF NOT EXISTS {{ident .Database}}.{{ident .ViewName}} {{.ClusterString}}
TO {{ident .Database}}.{{ident .TableName}}
AS SELECT
    TraceId,
    min(toDateTime(Timestamp)) AS Start,
    max(toDateTime(Timestamp)) AS End
FROM {{ident .Database}}.{{ident .SourceTableName}}
WHERE TraceId != ''
GROUP BY TraceId
