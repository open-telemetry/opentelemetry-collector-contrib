# OpsRamp Metrics Exporter

| Status                   |                       |
| ------------------------ | --------------------- |
| Stability                | [alpha]               |
| Supported pipeline types | metrics               |
| Distributions            | [contrib]             |

The OpsRamp Metrics exporter converts OpenTelemetry metrics to a simpler format with the following structure:

```go
type OpsRampMetric struct {
    MetricName string             // Name of the metric
    Value      float64            // Value of the metric
    Timestamp  int64              // Timestamp in milliseconds
    Labels     map[string]string  // Labels (attributes) for the metric
}
```

This exporter is useful for integrating with OpsRamp's metrics monitoring system that requires this specific format of metrics data.

## Configuration

```yaml
exporters:
  opsrampmetrics:
    # timeout is the timeout for sending data to the backend (default: 10s)
    timeout: 10s
    
    # add_metric_suffixes controls whether metric suffixes are added for metric types like histograms
    # and summaries (e.g. "_count", "_sum", "_bucket"). Default is true.
    add_metric_suffixes: true
    
    # resource_to_telemetry_conversion determines if resource attributes are added as metric labels
    resource_to_telemetry_conversion:
      enabled: true
    
    # sending_queue defines configuration for the queue used to buffer data before sending it to the backend.
    sending_queue:
      enabled: true
      num_consumers: 10
      queue_size: 5000
    
    # retry_on_failure defines configuration for retrying failed requests.
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 5m
```

## Metric Conversion Details

Each OTLP metric data point is converted to one or more OpsRampMetric instances:

### Gauge Metrics
Each data point in a gauge becomes one OpsRampMetric with the metric name, value, timestamp, and attributes.

### Sum Metrics
Each data point in a sum becomes one OpsRampMetric. Additional information about monotonicity and aggregation temporality is added as labels.

### Histogram Metrics
Each histogram data point is expanded into multiple OpsRampMetric instances:
- One for the sum (if available)
- One for the count
- One for each bucket

### Summary Metrics
Each summary data point is expanded into multiple OpsRampMetric instances:
- One for the sum
- One for the count
- One for each quantile value

## Example Output

For a simple gauge metric:
```json
{
  "MetricName": "system.cpu.usage",
  "Value": 0.724,
  "Timestamp": 1622622289432,
  "Labels": {
    "cpu": "0",
    "state": "idle",
    "host.name": "my-host"
  }
}
```

For a histogram metric bucket:
```json
{
  "MetricName": "http.server.request.duration_bucket",
  "Value": 123,
  "Timestamp": 1622622289432,
  "Labels": {
    "le": "0.1",
    "method": "GET",
    "path": "/api/users",
    "metric_type": "histogram_bucket"
  }
}
```

[alpha]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#alpha
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib