# Cassandra Exporter

| Status                   |           |
|--------------------------|-----------|
| Stability                | [alpha]   |
| Supported pipeline types | traces    |
| Distributions            | [contrib] |

## Configuration options

The following settings can be optionally configured:

- `dsn` The Cassandra server DSN (Data Source Name), for example `127.0.0.1`.
  reference: [https://pkg.go.dev/github.com/gocql/gocql](https://pkg.go.dev/github.com/gocql/gocql)
- `keyspace` (default = otel): The keyspace name.
- `trace_table` (default = otel_spans): The table name for traces.

## Example

```yaml
receivers:
  examplereceiver:
processors:
  batch:
    timeout: 5s
    send_batch_size: 100000
exporters:
  cassandra:
    dsn: 127.0.0.1
    keyspace: "otel"
    trace_table: "otel_spans"
service:
  pipelines:
    logs:
      receivers: [ examplereceiver ]
      processors: [ batch ]
      exporters: [ cassandra ]
```