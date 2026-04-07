# Alibaba Cloud MySQL DuckDB Exporter

| Status        |           |
| ------------- |-----------|
| Stability     | [alpha]: traces, logs, metrics |
| Distributions | [contrib] |
| Code Owners   | [@wangype](https://www.github.com/wangype) |

[alpha]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#alpha
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib

This exporter supports sending OpenTelemetry data (logs, traces, and metrics) to
[Alibaba Cloud RDS MySQL with DuckDB engine](https://www.alibabacloud.com/help/en/rds/apsaradb-rds-for-mysql/overview-for-duckdb-based-analytical-primary-instance).

> **Alibaba Cloud RDS MySQL DuckDB** is built by the Alibaba [AliSQL](https://github.com/AliSQL/AliSQL) team. It integrates the DuckDB analytical engine into Alibaba Cloud RDS MySQL, combining the best of both worlds:
>
> - **Native MySQL syntax** — fully compatible with the MySQL protocol and SQL dialect. Existing tools, drivers, and workflows work out of the box.
> - **DuckDB columnar storage** — data is stored in DuckDB's compressed columnar format, delivering significant storage savings compared to traditional InnoDB row storage.
> - **High-performance analytics** — leverages DuckDB's vectorized execution engine for fast analytical queries (aggregations, filtering, JOIN) over large volumes of observability data.
>
> This makes it an ideal backend for OpenTelemetry: write with standard MySQL INSERTs, query with familiar SQL, and enjoy columnar compression and analytical speed — without managing a separate OLAP system.

Note:
Always add a [batch processor](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/batchprocessor) to your collector pipeline.
The exporter uses batch multi-value `INSERT` to maximize throughput — larger batches mean fewer network round-trips and better performance.

## Configuration Options

The following settings are required:

- `endpoint` (no default): The MySQL DSN in [go-sql-driver/mysql](https://github.com/go-sql-driver/mysql#dsn-data-source-name) format.

  Format: `user:password@tcp(host:port)/dbname?params`

  Example for Alibaba Cloud RDS:
  ```
  root:password@tcp(rm-xxx.mysql.rds.aliyuncs.com:3306)/otel?tls=true
  ```

Connection options:

- `database` (default = `otel`): The database name. The exporter will create this database automatically if `create_schema` is enabled.
- `create_schema` (default = `true`): When set to true, the exporter will run DDL to create the database and tables on startup. (See [Schema Management](#schema-management))
- `ttl` (default = `0`, disabled): The data time-to-live. When set (e.g. `72h`, `168h`, `720h`), the exporter creates MySQL [scheduled EVENTs](https://dev.mysql.com/doc/refman/8.0/en/create-event.html) that periodically delete expired data. See [TTL / Data Retention](#ttl--data-retention) for details and prerequisites.

Table names:

- `logs_table_name` (default = `otel_logs`): The table name for logs.
- `traces_table_name` (default = `otel_traces`): The table name for traces.
- `metrics_table_name` (default = `otel_metrics`): The base table name prefix for metrics. The actual table names will be `<prefix>_gauge`, `<prefix>_sum`, `<prefix>_histogram`, and `<prefix>_summary`.

Processing:

- `timeout` (default = `5s`): The timeout for every attempt to send data to the backend.
- `sending_queue`
    - `enabled` (default = `true`)
    - `num_consumers` (default = `10`): Number of consumers that dequeue batches; ignored if `enabled` is `false`.
    - `queue_size` (default = `1000`): Maximum number of batches kept in memory before dropping data.
- `retry_on_failure`
    - `enabled` (default = `true`)
    - `initial_interval` (default = `5s`): Time to wait after the first failure before retrying; ignored if `enabled` is `false`.
    - `max_interval` (default = `30s`): The upper bound on backoff; ignored if `enabled` is `false`.
    - `max_elapsed_time` (default = `300s`): The maximum amount of time spent trying to send a batch; ignored if `enabled` is `false`.

## Table Schema

The exporter auto-creates the following tables when `create_schema` is `true`. All attribute fields use MySQL `JSON` type for flexible querying via `JSON_EXTRACT`.

### Logs Table (`otel_logs`)

| Column | Type | Description |
|--------|------|-------------|
| timestamp | DATETIME(6) | Log timestamp (microsecond precision) |
| trace_id | VARCHAR(32) | Associated trace ID |
| span_id | VARCHAR(16) | Associated span ID |
| trace_flags | TINYINT UNSIGNED | Trace flags |
| severity_text | VARCHAR(32) | Severity level text (e.g. "ERROR") |
| severity_number | TINYINT UNSIGNED | Severity level number |
| service_name | VARCHAR(256) | Service name from resource attributes |
| body | TEXT | Log body content |
| resource_attributes | JSON | Resource attributes |
| scope_name | VARCHAR(256) | Instrumentation scope name |
| scope_version | VARCHAR(64) | Instrumentation scope version |
| scope_attributes | JSON | Scope attributes |
| log_attributes | JSON | Log record attributes |

### Traces Table (`otel_traces`)

| Column | Type | Description |
|--------|------|-------------|
| timestamp | DATETIME(6) | Span start timestamp |
| trace_id | VARCHAR(32) | Trace ID |
| span_id | VARCHAR(16) | Span ID |
| parent_span_id | VARCHAR(16) | Parent span ID |
| span_name | VARCHAR(512) | Span name (operation name) |
| span_kind | VARCHAR(32) | Span kind (Client, Server, Internal, etc.) |
| service_name | VARCHAR(256) | Service name |
| resource_attributes | JSON | Resource attributes |
| span_attributes | JSON | Span attributes |
| duration | BIGINT UNSIGNED | Duration in nanoseconds |
| status_code | VARCHAR(32) | Status code (Ok, Error, Unset) |
| status_message | TEXT | Status message |
| events | JSON | Span events as JSON array |
| links | JSON | Span links as JSON array |

### Metrics Tables

Metrics are stored in separate tables by type, all prefixed with `metrics_table_name`:

| Metric Type | Table Suffix | Extra Columns |
|-------------|-------------|---------------|
| Gauge | `_gauge` | `value` |
| Sum | `_sum` | `value`, `aggregation_temporality`, `is_monotonic` |
| Histogram | `_histogram` | `count`, `sum`, `bucket_counts`, `explicit_bounds`, `min`, `max` |
| Summary | `_summary` | `count`, `sum`, `quantile_values` |

Common columns across all metric tables: `service_name`, `metric_name`, `metric_description`, `metric_unit`, `time_unix`, `attributes` (JSON), `resource_attributes` (JSON), `exemplars` (JSON).

## Query Examples

### Logs

- Get log severity count time series.

```sql
SELECT DATE_FORMAT(timestamp, '%Y-%m-%d %H:%i:00') AS minute,
       severity_text,
       COUNT(*) AS cnt
FROM otel_logs
WHERE timestamp >= NOW() - INTERVAL 1 HOUR
GROUP BY minute, severity_text
ORDER BY minute;
```

- Find recent logs by service.

```sql
SELECT timestamp, service_name, severity_text, body
FROM otel_logs
WHERE service_name = 'my-service'
  AND timestamp >= NOW() - INTERVAL 1 HOUR
ORDER BY timestamp DESC
LIMIT 100;
```

- Find logs with specific attribute.

```sql
SELECT timestamp, body,
       JSON_EXTRACT(log_attributes, '$.http.method') AS method
FROM otel_logs
WHERE JSON_EXTRACT(log_attributes, '$.http.status_code') = '500'
  AND timestamp >= NOW() - INTERVAL 1 HOUR
LIMIT 100;
```

- Find logs with body containing a keyword.

```sql
SELECT timestamp, body
FROM otel_logs
WHERE body LIKE '%error%'
  AND timestamp >= NOW() - INTERVAL 1 HOUR
LIMIT 100;
```

- Find logs with body JSON extract.

```sql
SELECT timestamp, body
FROM otel_logs
WHERE JSON_EXTRACT(body, '$.bytes') > 1000
  AND timestamp >= NOW() - INTERVAL 1 HOUR
LIMIT 100;
```

### Traces

- Find traces by trace ID.

```sql
SELECT timestamp, trace_id, span_id, span_name,
       service_name, duration / 1e6 AS duration_ms, status_code
FROM otel_traces
WHERE trace_id = '01020300000000000000000000000000'
ORDER BY timestamp;
```

- Find spans with specific service and attribute.

```sql
SELECT timestamp, trace_id, span_id, span_name,
       service_name, duration / 1e6 AS duration_ms
FROM otel_traces
WHERE service_name = 'my-service'
  AND JSON_EXTRACT(span_attributes, '$.http.method') = '"GET"'
  AND timestamp >= NOW() - INTERVAL 1 HOUR
LIMIT 100;
```

- Find error spans.

```sql
SELECT timestamp, trace_id, span_name, service_name,
       status_message
FROM otel_traces
WHERE status_code = 'Error'
  AND timestamp >= NOW() - INTERVAL 1 HOUR
LIMIT 100;
```

- Find slow spans (duration > 1 second).

```sql
SELECT timestamp, trace_id, span_id, span_name,
       service_name, duration / 1e9 AS duration_sec
FROM otel_traces
WHERE duration > 1000000000
  AND timestamp >= NOW() - INTERVAL 1 HOUR
ORDER BY duration DESC
LIMIT 100;
```

### Metrics

Metrics data is stored in different tables depending on their types. The tables have a suffix to distinguish the metric type:

| Metric Type | Table Name |
|-------------|-----------|
| Gauge | `otel_metrics_gauge` |
| Sum | `otel_metrics_sum` |
| Histogram | `otel_metrics_histogram` |
| Summary | `otel_metrics_summary` |

- Query gauge metrics.

```sql
SELECT time_unix, metric_name, value,
       JSON_EXTRACT(attributes, '$.cpu') AS cpu
FROM otel_metrics_gauge
WHERE metric_name = 'cpu.utilization'
  AND time_unix >= NOW() - INTERVAL 1 HOUR
ORDER BY time_unix;
```

- Query sum metrics (counters) with attribute filter.

```sql
SELECT time_unix, metric_name, value
FROM otel_metrics_sum
WHERE metric_name = 'request.count'
  AND service_name = 'my-service'
ORDER BY time_unix;
```

- Query histogram bucket distribution.

```sql
SELECT time_unix, metric_name, count, sum,
       JSON_EXTRACT(bucket_counts, '$') AS buckets,
       JSON_EXTRACT(explicit_bounds, '$') AS bounds
FROM otel_metrics_histogram
WHERE metric_name = 'request.duration'
ORDER BY time_unix DESC
LIMIT 100;
```

## Performance

The exporter uses **batch multi-value INSERT** for high throughput:

```sql
INSERT INTO table (col1, col2, ...) VALUES (?, ?, ...), (?, ?, ...), ...
```

Each batch contains up to 500 rows (configurable internally), reducing network round-trips compared to per-row inserts.

It is recommended to use the [batch processor](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/batchprocessor)
in your pipeline to maximize batch sizes sent to the exporter. A `send_batch_size` of 5000–10000 is a good starting point.

## Schema Management

By default, the exporter will create the database and tables under the names defined in the config.
This is fine for simple deployments, but for production workloads, it is recommended that you manage your own schema by setting `create_schema` to `false`.
This prevents each exporter process from racing to create the database and tables, and makes it easier to upgrade the exporter in the future.

In this mode, the only SQL sent to your server will be `INSERT` statements.

The default DDL used by the exporter can be found in `internal/sqltemplates/`.
Be sure to customize the indexes, partitioning, and other MySQL optimizations to fit your deployment.
Column names and types must match the exporter's `INSERT` statements to preserve compatibility.

## TTL / Data Retention

The exporter supports automatic data expiration via the `ttl` config option. When `ttl` is set to a non-zero duration, the exporter creates MySQL [scheduled EVENTs](https://dev.mysql.com/doc/refman/8.0/en/create-event.html) that run every hour to delete rows older than the specified TTL.

Example: `ttl: 72h` creates events that delete data older than 3 days.

### Prerequisites

1. **Enable `event_scheduler`** on the MySQL server. For Alibaba Cloud RDS, set the parameter `event_scheduler = ON` in the RDS parameter group.

2. **Grant EVENT privilege** to the database user:
   ```sql
   GRANT EVENT ON `otel`.* TO 'your_user'@'%';
   ```

### How it works

On startup, the exporter creates one `CREATE EVENT IF NOT EXISTS` per table:

```sql
CREATE EVENT IF NOT EXISTS `otel`.`ttl_otel_logs`
ON SCHEDULE EVERY 1 HOUR
DO
  DELETE FROM `otel`.`otel_logs`
  WHERE timestamp < DATE_SUB(NOW(), INTERVAL 259200 SECOND)
  LIMIT 100000;
```

Each execution deletes up to 100,000 rows to avoid long-running transactions. If the table has a large backlog of expired data, it will be cleaned up incrementally over multiple runs.

To inspect the created events:
```sql
SHOW EVENTS IN otel;
```

To manually disable TTL cleanup:
```sql
DROP EVENT IF EXISTS `otel`.`ttl_otel_logs`;
```

## Example Config

This example shows how to configure the exporter to send data to an Alibaba Cloud RDS MySQL DuckDB instance.
The exporter will create the database and tables if they don't exist. Data older than 72 hours is automatically cleaned up.

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

processors:
  batch:
    send_batch_size: 10000
    timeout: 5s

exporters:
  alibabacloudmysqlduckdb:
    endpoint: root:password@tcp(rm-xxx.mysql.rds.aliyuncs.com:3306)/otel
    database: otel
    create_schema: true
    ttl: 72h
    logs_table_name: otel_logs
    traces_table_name: otel_traces
    metrics_table_name: otel_metrics
    timeout: 10s
    sending_queue:
      enabled: true
      num_consumers: 10
      queue_size: 1000
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s

service:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [alibabacloudmysqlduckdb]
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [alibabacloudmysqlduckdb]
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [alibabacloudmysqlduckdb]
```

## Contributing

Before contributing, review the contribution guidelines in [CONTRIBUTING.md](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/CONTRIBUTING.md).

### Integration Tests

Integration tests require a running MySQL instance. Set the `MYSQL_DSN` environment variable and run:

```sh
MYSQL_DSN="user:pass@tcp(host:3306)/otel_test" go test -tags integration -v ./...
```

*Note: Make sure integration tests pass after making changes to SQL.*
