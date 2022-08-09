# ClickHouse Exporter


| Status                   |           |
| ------------------------ |-----------|
| Stability                | [alpha]    |
| Supported pipeline types | logs      |
| Distributions            | [contrib] |

This exporter supports sending OpenTelemetry logs to [ClickHouse](https://clickhouse.com/). It will also support spans and metrics in the future.
> ClickHouse is an open-source, high performance columnar OLAP database management system for real-time analytics using SQL.
> Throughput can be measured in rows per second or megabytes per second. 
> If the data is placed in the page cache, a query that is not too complex is processed on modern hardware at a speed of approximately 2-10 GB/s of uncompressed data on a single server.
> If 10 bytes of columns are extracted, the speed is expected to be around 100-200 million rows per second.

Note:
Always add [batch-processor](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/batchprocessor) to collector pipeline, as [ClickHouse document says:](https://clickhouse.com/docs/en/introduction/performance/#performance-when-inserting-data) 
> We recommend inserting data in packets of at least 1000 rows, or no more than a single request per second. When inserting to a MergeTree table from a tab-separated dump, the insertion speed can be from 50 to 200 MB/s.

## User Cases

1. Use [Grafana Clickhouse datasource](https://grafana.com/grafana/plugins/grafana-clickhouse-datasource/) or
[vertamedia-clickhouse-datasource](https://grafana.com/grafana/plugins/vertamedia-clickhouse-datasource/) to make dashboard.
Support time-series graph, table and logs.

2. Analyze logs via powerful clickhouse SQL.

- Get log severity count time series.
```clickhouse
SELECT toDateTime(toStartOfInterval(Timestamp, INTERVAL 60 second)) as time, SeverityText, count() as count
FROM otel_logs
WHERE time >= NOW() - INTERVAL 1 HOUR
GROUP BY SeverityText, time
ORDER BY time;
```
- Find any log.
```clickhouse
SELECT Timestamp as log_time, Body
FROM otel_logs 
WHERE Timestamp >= NOW() - INTERVAL 1 HOUR
Limit 100;
```
- Find log with specific service.
```clickhouse
SELECT Timestamp as log_time, Body
FROM otel_logs 
WHERE ServiceName = 'clickhouse-exporter' AND Timestamp >= NOW() - INTERVAL 1 HOUR
Limit 100;
```
- Find log with specific attribute.
```clickhouse
SELECT Timestamp as log_time, Body
FROM otel_logs 
WHERE LogAttributes['container_name'] = '/example_flog_1' AND Timestamp >= NOW() - INTERVAL 1 HOUR
Limit 100;
```
- Find log with body contain string token.
```clickhouse
SELECT Timestamp as log_time, Body
FROM otel_logs 
WHERE hasToken(Body, 'http') AND Timestamp >= NOW() - INTERVAL 1 HOUR
Limit 100;
```
- Find log with body contain string.
```clickhouse
SELECT Timestamp as log_time, Body
FROM otel_logs 
WHERE Body like '%http%' AND Timestamp >= NOW() - INTERVAL 1 HOUR
Limit 100;
```
- Find log with body regexp match string.
```clickhouse
SELECT Timestamp as log_time, Body
FROM otel_logs 
WHERE match(Body, 'http') AND Timestamp >= NOW() - INTERVAL 1 HOUR
Limit 100;
```
- Find log with body json extract.
```clickhouse
SELECT Timestamp as log_time, Body
FROM otel_logs 
WHERE JSONExtractFloat(Body, 'bytes')>1000 AND Timestamp >= NOW() - INTERVAL 1 HOUR
Limit 100;
```

## Performance Guide

A single clickhouse instance with 32 CPU cores and 128 GB RAM can handle around 20 TB (20 Billion) logs per day, 
the data compression ratio is 7 ~ 11, the compressed data store in disk is 1.8 TB ~ 2.85 TB,
add more clickhouse node to cluster can increase linearly.

The otel-collector with `otlp receiver/batch processor/clickhouse tcp exporter` can process 
around 40k/s logs entry per CPU cores, add more collector node can increase linearly.

## Configuration options

The following settings are required:

- `dsn` (no default): The ClickHouse server DSN (Data Source Name), for example `tcp://127.0.0.1:9000?username=user&password=qwerty&database=default`
   For tcp protocol reference: [ClickHouse/clickhouse-go#dsn](https://github.com/ClickHouse/clickhouse-go#dsn).
   For http protocol reference: [mailru/go-clickhouse/#dsn](https://github.com/mailru/go-clickhouse/#dsn).

The following settings can be optionally configured:

- `ttl_days` (defaul t= 0): The data time-to-live in days, 0 means no ttl.
- `logs_table_name` (default = otel_logs): The table name for logs.
- `timeout` (default = 5s): The timeout for every attempt to send data to the backend.
- `sending_queue`
  - `queue_size` (default = 5000): Maximum number of batches kept in memory before dropping data.
- `retry_on_failure`
    - `enabled` (default = true)
    - `initial_interval` (default = 5s): The Time to wait after the first failure before retrying; ignored if `enabled` is `false`
    - `max_interval` (default = 30s): The upper bound on backoff; ignored if `enabled` is `false`
    - `max_elapsed_time` (default = 300s): The maximum amount of time spent trying to send a batch; ignored if `enabled` is `false`

## Example

```yaml
receivers:
  examplereceiver:
processors:
  batch:
    timeout: 5s
    send_batch_size: 100000
exporters:
  clickhouse:
    dsn: tcp://127.0.0.1:9000/default
    ttl_days: 3
    timeout: 5s
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s
service:
  pipelines:
    logs:
      receivers: [examplereceiver]
      processors: [batch]
      exporters: [clickhouse]
```

## Schema

```clickhouse
CREATE TABLE otel_logs
(
    `Timestamp` DateTime64(9) CODEC(Delta, ZSTD(1)),
    `TraceId` String CODEC(ZSTD(1)),
    `SpanId` String CODEC(ZSTD(1)),
    `TraceFlags` UInt32 CODEC(ZSTD(1)),
    `SeverityText` LowCardinality(String) CODEC(ZSTD(1)),
    `SeverityNumber` Int32 CODEC(ZSTD(1)),
    `ServiceName` LowCardinality(String) CODEC(ZSTD(1)),
    `Body` String CODEC(ZSTD(1)),
    `ResourceAttributes` Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    `LogAttributes` Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    INDEX idx_trace_id TraceId TYPE bloom_filter(0.001) GRANULARITY 1,
    INDEX idx_res_attr_key mapKeys(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_res_attr_value mapValues(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_log_attr_key mapKeys(LogAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_log_attr_value mapValues(LogAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_body Body TYPE tokenbf_v1(32768, 3, 0) GRANULARITY 1
)
    ENGINE = MergeTree
        PARTITION BY toDate(Timestamp)
        ORDER BY (ServiceName, SeverityText, toUnixTimestamp(Timestamp), TraceId)
        TTL toDateTime(Timestamp) + toIntervalDay(3)
        SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;
```

[alpha]:https://github.com/open-telemetry/opentelemetry-collector#alpha
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
