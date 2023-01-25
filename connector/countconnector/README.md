# Count Connector

| Status                   |                                                           |
|------------------------- |---------------------------------------------------------- |
| Stability                | [in development]                                          |
| Supported pipeline types | See [Supported Pipeline Types](#supported-pipeline-types) |
| Distributions            | []                                                        |

<<<<<<< HEAD
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
=======
The `countconnector` can be used to count spans, data points, or log records.

## Supported Pipeline Types

Connectors are used as _both_ an exporter and as a receiver, in separate pipelines.
The pipeline in which a connector is used as an exporter is referred to below
as the "Exporter pipeline". Likewise, the pipeline in which the connector is
used as a receiver is referred to below as the "Receiver pipeline".

| Exporter pipeline | Receiver pipeline | Description                        | Default Metric Name       |
| ----------------- | ----------------- | ---------------------------------- | ------------------------- |
| traces            | metrics           | Counts the number of spans.        | `trace.span.count`        |
| metrics           | metrics           | Counts the number of log records.  | `metric.data_point.count` |
| logs              | metrics           | Counts the number of data points.  | `log.record.count`        |

## Configuration

Connectors are defined within a dedicated `connectors` section at the top level of the collector config.

The count connector may be used with default settings.

```yaml
receivers:
  foo:
exporters:
  bar:
connectors:
  count:
```

Optionally, the names of the metrics emitted by the connector may be customized.
>>>>>>> 8811b83ca2 ([connector/count] Add outline and documentation)

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

<<<<<<< HEAD
### Example Usage

Count spans, only exporting the count metrics.
=======
### Use the connector in pipelines

A connector _is_ an exporter _and_ a receiver. It must be used as both, in separate pipelines.
>>>>>>> 8811b83ca2 ([connector/count] Add outline and documentation)

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

<<<<<<< HEAD
Count spans, exporting both the original traces and the count metrics.
=======
Connectors can be used alongside other exporters.
>>>>>>> 8811b83ca2 ([connector/count] Add outline and documentation)

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

<<<<<<< HEAD
Count spans, data points, and log records, exporting count metrics to a separate backend.
=======
Connectors can be used alongside other receivers.

```yaml
receivers:
  foo/traces:
  foo/metrics:
exporters:
  bar:
connectors:
  count:
service:
  pipelines:
    traces:
      receivers: [foo/traces]
      exporters: [count]
    metrics:
      receivers: [foo/metrics, count]
      exporters: [bar]
```

A connector can be an exporter from multiple pipelines.
>>>>>>> 8811b83ca2 ([connector/count] Add outline and documentation)

```yaml
receivers:
  foo/traces:
  foo/metrics:
  foo/logs:
exporters:
<<<<<<< HEAD
  bar/all_types:
  bar/counts_only:
=======
  bar/traces_backend:
  bar/metrics_backend:
  bar/logs_backend:
>>>>>>> 8811b83ca2 ([connector/count] Add outline and documentation)
connectors:
  count:
service:
  pipelines:
    traces:
      receivers: [foo/traces]
<<<<<<< HEAD
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
=======
      exporters: [bar/traces_backend, count]
    metrics:
      receivers: [foo/metrics]
      exporters: [bar/metrics_backend, count]
    logs:
      receivers: [foo/logs]
      exporters: [bar/logs_backend, count]
    metrics/counts:
      receivers: [count]
      exporters: [bar/metrics_backend]
```

A connector can be a receiver in multiple pipelines.

```yaml
receivers:
  foo/traces:
  foo/metrics:
exporters:
  bar/traces_backend:
  bar/metrics_backend:
  bar/metrics_backend/2:
connectors:
  count:
service:
  pipelines:
    traces:
      receivers: [foo/traces]
      exporters: [bar/traces_backend, count]
    metrics:
      receivers: [count]
      exporters: [bar/metrics_backend]
    metrics/2:
      receivers: [count]
      exporters: [bar/metrics_backend/2]
```

Multiple connectors can be used in sequence.

```yaml
receivers:
  foo:
exporters:
  bar:
connectors:
  count:
  count/the_counts:
service:
  pipelines:
    traces:
      receivers: [foo]
      exporters: [count]
    metrics:
      receivers: [count]
      exporters: [bar/metrics_backend, count/the_counts]
    metrics/count_the_counts:
      receivers: [count/the_counts]
      exporters: [bar]
```

A connector can only be used in a pair of pipelines when it supports the combination of _exporter type_ and _receiver type_.

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
    logs:
      receivers: [count] # Invalid. The count connector can only be used as a receiver in metrics pipelines.
      exporters: [bar]
```

[in development]:https://github.com/open-telemetry/opentelemetry-collector#in-development
>>>>>>> 8811b83ca2 ([connector/count] Add outline and documentation)
