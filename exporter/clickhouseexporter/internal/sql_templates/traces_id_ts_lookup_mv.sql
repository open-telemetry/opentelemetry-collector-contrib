CREATE MATERIALIZED VIEW IF NOT EXISTS "%s"."%s_trace_id_ts_mv" %s
TO "%s"."%s_trace_id_ts"
AS SELECT
              TraceId,
              min(Timestamp) as Start,
              max(Timestamp) as End
   FROM "%s"."%s"
   WHERE TraceId != ''
   GROUP BY TraceId
