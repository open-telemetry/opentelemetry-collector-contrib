# Count Connector

| Status                   |                                                           |
|------------------------- |---------------------------------------------------------- |
| Stability                | [in development]                                          |
| Supported pipeline types | See [Supported Pipeline Types](#supported-pipeline-types) |
| Distributions            | []                                                        |

The `count` connector can be used to count spans, data points, or log records.

## Supported Pipeline Types

| [Exporter Pipeline Type] | [Receiver Pipeline Type] | Description                        | Default Metric Name       |
| ------------------------ | ------------------------ | ---------------------------------- | ------------------------- |
| traces                   | metrics                  | Counts the number of spans.        | `trace.span.count`        |
| metrics                  | metrics                  | Counts the number of log records.  | `metric.data_point.count` |
| logs                     | metrics                  | Counts the number of data points.  | `log.record.count`        |

## Configuration

If you are not already familiar with connectors, you may find it helpful to first visit the [Connectors README].

The count connector may be used with default settings. Optionally, the names of the emitted metrics may be customized.

```yaml
receivers:
  foo:
exporters:
  bar:
connectors:
  count:
    traces:
      metric_name: my.span.count
    metrics:
      metric_name: my.data_point.count
    logs:
      metric_name: my.log_record.count
```

### Example Usage

Count spans, only exporting the count metrics.

```yaml
receivers:
  foo:
exporters:
  bar:
connectors:
  count:
service:
  pipelines:
    traces:
      receivers: [foo]
      exporters: [count]
    metrics:
      receivers: [count]
      exporters: [bar]
```

Count spans, exporting both the original traces and the count metrics.

```yaml
receivers:
  foo:
exporters:
  bar/traces_backend:
  bar/metrics_backend:
connectors:
  count:
service:
  pipelines:
    traces:
      receivers: [foo]
      exporters: [bar/traces_backend, count]
    metrics:
      receivers: [count]
      exporters: [bar/metrics_backend]
```

Count spans, data points, and log records, exporting count metrics to a separate backend.

```yaml
receivers:
  foo/traces:
  foo/metrics:
  foo/logs:
exporters:
  bar/all_types:
  bar/counts_only:
connectors:
  count:
service:
  pipelines:
    traces:
      receivers: [foo/traces]
      exporters: [bar/all_types, count]
    metrics:
      receivers: [foo/metrics]
      exporters: [bar/all_types, count]
    logs:
      receivers: [foo/logs]
      exporters: [bar/all_types, count]
    metrics/counts:
      receivers: [count]
      exporters: [bar/counts_only]
```

[in development]:https://github.com/open-telemetry/opentelemetry-collector#in-development
[Connectors README]:https://github.com/open-telemetry/opentelemetry-collector/blob/main/connector/README.md
[Exporter Pipeline Type]:../README.md#exporter-pipeline-type
[Receiver Pipeline Type]:../README.md#receiver-pipeline-type
