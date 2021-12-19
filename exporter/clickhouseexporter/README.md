# ClickHouse Exporter

This exporter supports sending OpenTelemetry logs to [ClickHouse](https://clickhouse.com/).

Note:
Always add [batchprocessor](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/batchprocessor) to collector pipeline, as Clickhouse [recommend](https://clickhouse.com/docs/en/introduction/performance/#performance-when-inserting-data ) inserting data in packets of at least 1000 rows, or no more than a single request per second.

## Configuration options

- `address` is the ClickHouse server address.
- `database` is the database name write data.
- `username`is the username connect to ClickHouse.
- `password` is the password connect to ClickHouse.
- `ca_file` is the ca_file connect to ClickHouse.
- `ttl` is the data ttl days.

## Example

```yaml
exporters:
  clickhouse:
    address: tcp://127.0.0.1:9000
    database: default
    username: username
    password: password
    ca_file: ca_file
    ttl: 3
```
