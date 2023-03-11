# Span Metrics Connector

| Status                   |                                                           |
|------------------------- |---------------------------------------------------------- |
| Stability                | [development]                                             |
| Supported pipeline types | See [Supported Pipeline Types](#supported-pipeline-types) |
| Distributions            | []                                                        |

## Supported Pipeline Types

| [Exporter Pipeline Type] | [Receiver Pipeline Type] |
| ------------------------ | ------------------------ |
| traces                   | metrics                  |

## Overview

Aggregates Request, Error and Duration (R.E.D) OpenTelemetry metrics from span data.

**Request** counts are computed as the number of spans seen per unique set of
dimensions, including Errors. Multiple metrics can be aggregated if, for instance,
a user wishes to view call counts just on `service.name` and `span.name`.

**Error** counts are computed from the Request counts which have an `Error` Status Code metric dimension.

**Duration** is computed from the difference between the span start and end times and inserted into the
relevant duration histogram time bucket for each unique set dimensions.

Each metric will have _at least_ the following dimensions because they are common
across all spans:

- `service.name`
- `span.name`
- `span.kind`
- `status.code`

## Configurations

If you are not already familiar with connectors, you may find it helpful to first
visit the [Connectors README].

The following settings can be optionally configured:

- `histogram` (default: `explicit_buckets`): Use to configure the type of histogram to record
  calculated from spans duration measurements.
  - `unit` (default: `ms`, allowed values: `ms`, `s`): The time unit for recording duration measurements.
  calculated from spans duration measurements.
  - `explicit`:
    - `buckets`: the list of durations defining the duration histogram buckets. Default
      buckets: `[2ms, 4ms, 6ms, 8ms, 10ms, 50ms, 100ms, 200ms, 400ms, 800ms, 1s, 1400ms, 2s, 5s, 10s, 15s]`
  - `exponential`:
    - `max_size` (default: 160) the maximum number of buckets per positive or negative number range.
- `dimensions`: the list of dimensions to add together with the default dimensions defined above.
  
  Each additional dimension is defined with a `name` which is looked up in the span's collection of attributes or
  resource attributes (AKA process tags) such as `ip`, `host.name` or `region`.
  
  If the `name`d attribute is missing in the span, the optional provided `default` is used.
  
  If no `default` is provided, this dimension will be **omitted** from the metric.
- `dimensions_cache_size`: the max items number of `metric_key_to_dimensions_cache`. If not provided, will
  use default value size `1000`.
- `aggregation_temporality`: Defines the aggregation temporality of the generated metrics. 
  One of either `AGGREGATION_TEMPORALITY_CUMULATIVE` or `AGGREGATION_TEMPORALITY_DELTA`.
  - Default: `AGGREGATION_TEMPORALITY_CUMULATIVE`
- `namespace`: Defines the namespace of the generated metrics. If `namespace` provided, generated metric name will be added `namespace.` prefix.

## Examples

The following is a simple example usage of the `spanmetrics` connector.

For configuration examples on other use cases, please refer to [More Examples](#more-examples).

The full list of settings exposed for this connector are documented [here](../../connector/spanmetricsconnector/config.go).

```yaml
receivers:
  nop:

exporters:
  nop:

connector:
  spanmetrics:
    histogram:
      explicit:
        buckets: [100us, 1ms, 2ms, 6ms, 10ms, 100ms, 250ms]
    dimensions:
      - name: http.method
        default: GET
      - name: http.status_code
    dimensions_cache_size: 1000
    aggregation_temporality: "AGGREGATION_TEMPORALITY_CUMULATIVE"     

service:
  pipelines:
    traces:
      receivers: [nop]
      exporters: [spanmetrics]
    metrics:
      receivers: [spanmetrics]
      exporters: [nop]
```

### More Examples

For more example configuration covering various other use cases, please visit the [testdata directory](../../connector/spanmetricsconnector/testdata).

[development]: https://github.com/open-telemetry/opentelemetry-collector#development
[Connectors README]:https://github.com/open-telemetry/opentelemetry-collector/blob/main/connector/README.md
[Exporter Pipeline Type]:https://github.com/open-telemetry/opentelemetry-collector/blob/main/connector/README.md#exporter-pipeline-type
[Receiver Pipeline Type]:https://github.com/open-telemetry/opentelemetry-collector/blob/main/connector/README.md#receiver-pipeline-type