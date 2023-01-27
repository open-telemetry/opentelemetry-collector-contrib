# ClickHouse Exporter

| Status                   |                       |
| ------------------------ |-----------------------|
| Stability                | [alpha]               |
| Supported pipeline types | traces, logs, metrics |
| Distributions            | [contrib]             |

This exporter supports sending OpenTelemetry data to [ClickHouse](https://clickhouse.com/). 
> ClickHouse is an open-source, high performance columnar OLAP database management system for real-time analytics using
> SQL.
> Throughput can be measured in rows per second or megabytes per second.
> If the data is placed in the page cache, a query that is not too complex is processed on modern hardware at a speed of
> approximately 2-10 GB/s of uncompressed data on a single server.
> If 10 bytes of columns are extracted, the speed is expected to be around 100-200 million rows per second.

Note:
Always
add [batch-processor](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/batchprocessor) to
collector pipeline,
as [ClickHouse document says:](https://clickhouse.com/docs/en/introduction/performance/#performance-when-inserting-data)
> We recommend inserting data in packets of at least 1000 rows, or no more than a single request per second. When
> inserting to a MergeTree table from a tab-separated dump, the insertion speed can be from 50 to 200 MB/s.

## User Cases

1. Use [Grafana Clickhouse datasource](https://grafana.com/grafana/plugins/grafana-clickhouse-datasource/) or
   [vertamedia-clickhouse-datasource](https://grafana.com/grafana/plugins/vertamedia-clickhouse-datasource/) to make
   dashboard.
   Support time-series graph, table and logs.

2. Analyze logs via powerful clickhouse SQL.

### Logs

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
WHERE ServiceName = 'clickhouse-exporter'
  AND Timestamp >= NOW() - INTERVAL 1 HOUR
Limit 100;
```

- Find log with specific attribute.

```clickhouse
SELECT Timestamp as log_time, Body
FROM otel_logs
WHERE LogAttributes['container_name'] = '/example_flog_1'
  AND Timestamp >= NOW() - INTERVAL 1 HOUR
Limit 100;
```

- Find log with body contain string token.

```clickhouse
SELECT Timestamp as log_time, Body
FROM otel_logs
WHERE hasToken(Body, 'http')
  AND Timestamp >= NOW() - INTERVAL 1 HOUR
Limit 100;
```

- Find log with body contain string.

```clickhouse
SELECT Timestamp as log_time, Body
FROM otel_logs
WHERE Body like '%http%'
  AND Timestamp >= NOW() - INTERVAL 1 HOUR
Limit 100;
```

- Find log with body regexp match string.

```clickhouse
SELECT Timestamp as log_time, Body
FROM otel_logs
WHERE match(Body, 'http')
  AND Timestamp >= NOW() - INTERVAL 1 HOUR
Limit 100;
```

- Find log with body json extract.

```clickhouse
SELECT Timestamp as log_time, Body
FROM otel_logs
WHERE JSONExtractFloat(Body, 'bytes') > 1000
  AND Timestamp >= NOW() - INTERVAL 1 HOUR
Limit 100;
```

### Traces

- Find spans with specific attribute.

```clickhouse
SELECT Timestamp as log_time,
       TraceId,
       SpanId,
       ParentSpanId,
       SpanName,
       SpanKind,
       ServiceName,
       Duration,
       StatusCode,
       StatusMessage,
       toString(SpanAttributes),
       toString(ResourceAttributes),
       toString(Events.Name),
       toString(Links.TraceId)
FROM otel_traces
WHERE ServiceName = 'clickhouse-exporter'
  AND SpanAttributes['peer.service'] = 'tracegen-server'
  AND Timestamp >= NOW() - INTERVAL 1 HOUR
Limit 100;
```

- Find traces with traceID (using time primary index and TraceID skip index).

```clickhouse
WITH
    '391dae938234560b16bb63f51501cb6f' as trace_id,
    (SELECT min(Start) FROM otel_traces_trace_id_ts WHERE TraceId = trace_id) as start,
    (SELECT max(End) + 1 FROM otel_traces_trace_id_ts WHERE TraceId = trace_id) as end
SELECT Timestamp as log_time,
       TraceId,
       SpanId,
       ParentSpanId,
       SpanName,
       SpanKind,
       ServiceName,
       Duration,
       StatusCode,
       StatusMessage,
       toString(SpanAttributes),
       toString(ResourceAttributes),
       toString(Events.Name),
       toString(Links.TraceId)
FROM otel_traces
WHERE TraceId = trace_id
  AND Timestamp >= start
  AND Timestamp <= end
Limit 100;
```

- Find spans is error.

```clickhouse
SELECT Timestamp as log_time,
       TraceId,
       SpanId,
       ParentSpanId,
       SpanName,
       SpanKind,
       ServiceName,
       Duration,
       StatusCode,
       StatusMessage,
       toString(SpanAttributes),
       toString(ResourceAttributes),
       toString(Events.Name),
       toString(Links.TraceId)
FROM otel_traces
WHERE ServiceName = 'clickhouse-exporter'
  AND StatusCode = 'STATUS_CODE_ERROR'
  AND Timestamp >= NOW() - INTERVAL 1 HOUR
Limit 100;
```

- Find slow spans.

```clickhouse
SELECT Timestamp as log_time,
       TraceId,
       SpanId,
       ParentSpanId,
       SpanName,
       SpanKind,
       ServiceName,
       Duration,
       StatusCode,
       StatusMessage,
       toString(SpanAttributes),
       toString(ResourceAttributes),
       toString(Events.Name),
       toString(Links.TraceId)
FROM otel_traces
WHERE ServiceName = 'clickhouse-exporter'
  AND Duration > 1 * 1e9
  AND Timestamp >= NOW() - INTERVAL 1 HOUR
Limit 100;
```

### Metrics

Metrics data is stored in different clickhouse tables depending on their types. The tables will have a suffix to
distinguish which type of metrics data is stored.

| Metrics Type          | Metrics Table          |
| --------------------- | ---------------------- |
| sum                   | _sum                   |
| gauge                 | _gauge                 |
| histogram             | _histogram             |
| exponential histogram | _exponential_histogram |
| summary               | _summary               |

Before you make a metrics query, you need to know the type of metric you wish to use. If your metrics come from
Prometheus(or someone else uses OpenMetrics protocol), you also need to know the
[compatibility](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/compatibility/prometheus_and_openmetrics.md#prometheus-and-openmetrics-compatibility)
between Prometheus(OpenMetrics) and OTLP Metrics.

- Find a sum metrics with name
```clickhouse
select TimeUnix,MetricName,Attributes,Value from otel_metrics_sum
where MetricName='calls_total' limit 100
```

- Find a sum metrics with name, attribute.
```clickhouse
select TimeUnix,MetricName,Attributes,Value from otel_metrics_sum
where MetricName='calls_total' and Attributes['service_name']='featureflagservice'
limit 100
```

The OTLP Metrics [define two type value for one datapoint](https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/metrics/v1/metrics.proto#L358),
clickhouse only use one value of float64 to store them.

## Performance Guide

A single clickhouse instance with 32 CPU cores and 128 GB RAM can handle around 20 TB (20 Billion) logs per day,
the data compression ratio is 7 ~ 11, the compressed data store in disk is 1.8 TB ~ 2.85 TB,
add more clickhouse node to cluster can increase linearly.

The otel-collector with `otlp receiver/batch processor/clickhouse tcp exporter` can process
around 40k/s logs entry per CPU cores, add more collector node can increase linearly.

## Configuration options

The following settings are required:

- `endpoint` (no default): The ClickHouse server endpoint, support multi host, for example:
  tcp protocol `tcp://ip1:port,ip2:port`
  http protocol `http://ip:port,ip2:port`

The following settings can be optionally configured:

- `username` (default = ): The authentication username.
- `password` (default = ): The authentication password.
- `ttl_days` (default = 0): The data time-to-live in days, 0 means no ttl.
- `database` (default = otel): The database name.
- `connection_params` (default = {}). Params is the extra connection parameters with map format. for example compression/dial_timeout.
- `logs_table_name` (default = otel_logs): The table name for logs.
- `traces_table_name` (default = otel_traces): The table name for traces.
- `metrics_table_name` (default = otel_metrics): The table name for metrics.
- `timeout` (default = 5s): The timeout for every attempt to send data to the backend.
- `sending_queue`
    - `queue_size` (default = 5000): Maximum number of batches kept in memory before dropping data.
- `retry_on_failure`
    - `enabled` (default = true)
    - `initial_interval` (default = 5s): The Time to wait after the first failure before retrying; ignored if `enabled`
      is `false`
    - `max_interval` (default = 30s): The upper bound on backoff; ignored if `enabled` is `false`
    - `max_elapsed_time` (default = 300s): The maximum amount of time spent trying to send a batch; ignored if `enabled`
      is `false`

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
    endpoint: tcp://127.0.0.1:9000
    database: otel
    ttl_days: 3
    logs_table: otel_logs
    traces_table: otel_traces
    metrics_table: otel_metrics
    timeout: 5s
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s
service:
  pipelines:
    logs:
      receivers: [ examplereceiver ]
      processors: [ batch ]
      exporters: [ clickhouse ]
```

## Schema

### Logs

```clickhouse
CREATE TABLE otel_logs
(
    `Timestamp`      DateTime64(9) CODEC (Delta, ZSTD(1)),
    `TraceId`        String CODEC (ZSTD(1)),
    `SpanId`         String CODEC (ZSTD(1)),
    `TraceFlags`     UInt32 CODEC (ZSTD(1)),
    `SeverityText` LowCardinality(String) CODEC (ZSTD(1)),
    `SeverityNumber` Int32 CODEC (ZSTD(1)),
    `ServiceName` LowCardinality(String) CODEC (ZSTD(1)),
    `Body`           String CODEC (ZSTD(1)),
    `ResourceAttributes` Map(LowCardinality(String), String) CODEC (ZSTD(1)),
    `LogAttributes` Map(LowCardinality(String), String) CODEC (ZSTD(1)),
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

### Traces

```clickhouse
CREATE TABLE otel.otel_traces
(
    `Timestamp`     DateTime64(9) CODEC (Delta(8), ZSTD(1)),
    `TraceId`       String CODEC (ZSTD(1)),
    `SpanId`        String CODEC (ZSTD(1)),
    `ParentSpanId`  String CODEC (ZSTD(1)),
    `TraceState`    String CODEC (ZSTD(1)),
    `SpanName` LowCardinality(String) CODEC (ZSTD(1)),
    `SpanKind` LowCardinality(String) CODEC (ZSTD(1)),
    `ServiceName` LowCardinality(String) CODEC (ZSTD(1)),
    `ResourceAttributes` Map(LowCardinality(String), String) CODEC (ZSTD(1)),
    `SpanAttributes` Map(LowCardinality(String), String) CODEC (ZSTD(1)),
    `Duration`      Int64 CODEC (ZSTD(1)),
    `StatusCode` LowCardinality(String) CODEC (ZSTD(1)),
    `StatusMessage` String CODEC (ZSTD(1)),
    `Events` Nested (
        `Timestamp` DateTime64(9),
        `Name` LowCardinality(String),
        `Attributes` Map(LowCardinality(String), String)
        ) CODEC(ZSTD(1)),
    `Links` Nested (
        `TraceId` String,
        `SpanId` String,
        `TraceState` String,
        `Attributes` Map(LowCardinality(String), String)
    ) CODEC(ZSTD(1)),
    INDEX idx_trace_id TraceId TYPE bloom_filter(0.001) GRANULARITY 1,
    INDEX idx_res_attr_key mapKeys(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_res_attr_value mapValues(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_span_attr_key mapKeys(SpanAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_span_attr_value mapValues(SpanAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_duration Duration TYPE minmax GRANULARITY 1
)
    ENGINE = MergeTree
        PARTITION BY toDate(Timestamp)
        ORDER BY (ServiceName, SpanName, toUnixTimestamp(Timestamp), TraceId)
        TTL toDateTime(Timestamp) + toIntervalDay(3)
        SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

CREATE TABLE otel_traces_trace_id_ts
(
    `TraceId` String CODEC (ZSTD(1)),
    `Start`   DateTime CODEC (ZSTD(1)),
    `End`     DateTime CODEC (ZSTD(1)),
    INDEX idx_trace_id TraceId TYPE bloom_filter(0.01) GRANULARITY 1
)
    ENGINE = MergeTree
        ORDER BY (TraceId, toUnixTimestamp(Start))
        TTL toDateTime(Start) + toIntervalDay(3)
        SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW otel.otel_traces_trace_id_ts_mv TO otel.otel_traces_trace_id_ts
AS
SELECT TraceId,
       min(toDateTime(Timestamp)) AS Start,
       max(toDateTime(Timestamp)) AS End
FROM otel.otel_traces
WHERE TraceId != ''
GROUP BY TraceId;
```

### Metrics

#### Gauge

```clickhouse
CREATE TABLE IF NOT EXISTS otel.otel_metrics_gauge (
    ResourceAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    ResourceSchemaUrl String CODEC(ZSTD(1)),
    ScopeName String CODEC(ZSTD(1)),
    ScopeVersion String CODEC(ZSTD(1)),
    ScopeAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    ScopeDroppedAttrCount UInt32 CODEC(ZSTD(1)),
    ScopeSchemaUrl String CODEC(ZSTD(1)),
    MetricName String CODEC(ZSTD(1)),
    MetricDescription String CODEC(ZSTD(1)),
    MetricUnit String CODEC(ZSTD(1)),
    Attributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    StartTimeUnix DateTime64(9) CODEC(Delta, ZSTD(1)),
    TimeUnix DateTime64(9) CODEC(Delta, ZSTD(1)),
    Value Float64 CODEC(ZSTD(1)),
    Flags UInt32 CODEC(ZSTD(1)),
    Exemplars Nested (
		FilteredAttributes Map(LowCardinality(String), String),
		TimeUnix DateTime64(9),
		Value Float64,
		SpanId String,
		TraceId String
    ) CODEC(ZSTD(1))
) ENGINE MergeTree()
TTL toDateTime(TimeUnix) + toIntervalDay(3)
PARTITION BY toDate(TimeUnix)
ORDER BY (MetricName, Attributes, toUnixTimestamp64Nano(TimeUnix))
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1;
```

#### Sum

```clickhouse
CREATE TABLE IF NOT EXISTS otel.otel_metrics_sum (
    ResourceAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    ResourceSchemaUrl String CODEC(ZSTD(1)),
    ScopeName String CODEC(ZSTD(1)),
    ScopeVersion String CODEC(ZSTD(1)),
    ScopeAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    ScopeDroppedAttrCount UInt32 CODEC(ZSTD(1)),
    ScopeSchemaUrl String CODEC(ZSTD(1)),
    MetricName String CODEC(ZSTD(1)),
    MetricDescription String CODEC(ZSTD(1)),
    MetricUnit String CODEC(ZSTD(1)),
    Attributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
	StartTimeUnix DateTime64(9) CODEC(Delta, ZSTD(1)),
	TimeUnix DateTime64(9) CODEC(Delta, ZSTD(1)),
	Value Float64 CODEC(ZSTD(1)),
	Flags UInt32  CODEC(ZSTD(1)),
    Exemplars Nested (
		FilteredAttributes Map(LowCardinality(String), String),
		TimeUnix DateTime64(9),
		Value Float64,
		SpanId String,
		TraceId String
    ) CODEC(ZSTD(1)),
    AggTemp Int32 CODEC(ZSTD(1)),
	IsMonotonic Boolean CODEC(Delta, ZSTD(1))
) ENGINE MergeTree()
TTL toDateTime(TimeUnix) + toIntervalDay(3)
PARTITION BY toDate(TimeUnix)
ORDER BY (MetricName, Attributes, toUnixTimestamp64Nano(TimeUnix))
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1;
```

#### Histogram

```clickhouse
CREATE TABLE IF NOT EXISTS otel.otel_metrics_histogram (
    ResourceAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    ResourceSchemaUrl String CODEC(ZSTD(1)),
    ScopeName String CODEC(ZSTD(1)),
    ScopeVersion String CODEC(ZSTD(1)),
    ScopeAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    ScopeDroppedAttrCount UInt32 CODEC(ZSTD(1)),
    ScopeSchemaUrl String CODEC(ZSTD(1)),
    MetricName String CODEC(ZSTD(1)),
    MetricDescription String CODEC(ZSTD(1)),
    MetricUnit String CODEC(ZSTD(1)),
    Attributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
	StartTimeUnix DateTime64(9) CODEC(Delta, ZSTD(1)),
	TimeUnix DateTime64(9) CODEC(Delta, ZSTD(1)),
    Count Int64 CODEC(Delta, ZSTD(1)),
    Sum Float64 CODEC(ZSTD(1)),
    BucketCounts Array(UInt64) CODEC(ZSTD(1)),
    ExplicitBounds Array(Float64) CODEC(ZSTD(1)),
	Exemplars Nested (
		FilteredAttributes Map(LowCardinality(String), String),
		TimeUnix DateTime64(9),
		Value Float64,
		SpanId String,
		TraceId String
    ) CODEC(ZSTD(1)),
    Flags UInt32 CODEC(ZSTD(1)),
    Min Float64 CODEC(ZSTD(1)),
    Max Float64 CODEC(ZSTD(1))
) ENGINE MergeTree()
TTL toDateTime(TimeUnix) + toIntervalDay(3)
PARTITION BY toDate(TimeUnix)
ORDER BY (MetricName, Attributes, toUnixTimestamp64Nano(TimeUnix))
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1;
```

#### Exponential histogram

```clickhouse
CREATE TABLE IF NOT EXISTS otel.otel_metrics_exponential_histogram (
    ResourceAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    ResourceSchemaUrl String CODEC(ZSTD(1)),
    ScopeName String CODEC(ZSTD(1)),
    ScopeVersion String CODEC(ZSTD(1)),
    ScopeAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    ScopeDroppedAttrCount UInt32 CODEC(ZSTD(1)),
    ScopeSchemaUrl String CODEC(ZSTD(1)),
    MetricName String CODEC(ZSTD(1)),
    MetricDescription String CODEC(ZSTD(1)),
    MetricUnit String CODEC(ZSTD(1)),
    Attributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
	StartTimeUnix DateTime64(9) CODEC(Delta, ZSTD(1)),
	TimeUnix DateTime64(9) CODEC(Delta, ZSTD(1)),
    Count Int64 CODEC(Delta, ZSTD(1)),
    Sum Float64 CODEC(ZSTD(1)),
    Scale Int32 CODEC(ZSTD(1)),
    ZeroCount UInt64 CODEC(ZSTD(1)),
	PositiveOffset Int32 CODEC(ZSTD(1)),
	PositiveBucketCounts Array(UInt64) CODEC(ZSTD(1)),
	NegativeOffset Int32 CODEC(ZSTD(1)),
	NegativeBucketCounts Array(UInt64) CODEC(ZSTD(1)),
	Exemplars Nested (
		FilteredAttributes Map(LowCardinality(String), String),
		TimeUnix DateTime64(9),
		Value Float64,
		SpanId String,
		TraceId String
    ) CODEC(ZSTD(1)),
    Flags UInt32  CODEC(ZSTD(1)),
    Min Float64 CODEC(ZSTD(1)),
    Max Float64 CODEC(ZSTD(1))
) ENGINE MergeTree()
TTL toDateTime(TimeUnix) + toIntervalDay(3)
PARTITION BY toDate(TimeUnix)
ORDER BY (MetricName, Attributes, toUnixTimestamp64Nano(TimeUnix))
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1;
```

#### Summary

```clickhouse
CREATE TABLE IF NOT EXISTS otel.otel_metrics_summary (
    ResourceAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    ResourceSchemaUrl String CODEC(ZSTD(1)),
    ScopeName String CODEC(ZSTD(1)),
    ScopeVersion String CODEC(ZSTD(1)),
    ScopeAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    ScopeDroppedAttrCount UInt32 CODEC(ZSTD(1)),
    ScopeSchemaUrl String CODEC(ZSTD(1)),
    MetricName String CODEC(ZSTD(1)),
    MetricDescription String CODEC(ZSTD(1)),
    MetricUnit String CODEC(ZSTD(1)),
    Attributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
	StartTimeUnix DateTime64(9) CODEC(Delta, ZSTD(1)),
	TimeUnix DateTime64(9) CODEC(Delta, ZSTD(1)),
    Count UInt64 CODEC(Delta, ZSTD(1)),
    Sum Float64 CODEC(ZSTD(1)),
    ValueAtQuantiles Nested(
		Quantile Float64,
		Value Float64
	) CODEC(ZSTD(1)),
    Flags UInt32  CODEC(ZSTD(1))
) ENGINE MergeTree()
TTL toDateTime(TimeUnix) + toIntervalDay(3)
PARTITION BY toDate(TimeUnix)
ORDER BY (MetricName, Attributes, toUnixTimestamp64Nano(TimeUnix))
SETTINGS index_granularity=8192, ttl_only_drop_parts = 1;
```


[alpha]:https://github.com/open-telemetry/opentelemetry-collector#alpha

[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
