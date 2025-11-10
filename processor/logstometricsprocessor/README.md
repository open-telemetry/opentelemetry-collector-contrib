# Logs to Metrics Processor

The Logs to Metrics Processor (`logstometricsprocessor`) extracts metrics from log records using OpenTelemetry Transformation Language (OTTL) expressions. It supports extracting Sum, Gauge, Histogram, and Exponential Histogram metrics from log records and can optionally forward or drop logs after processing.

## Overview

This processor is designed to convert log data into metrics, enabling you to:
- Extract metrics from log records using flexible OTTL expressions
- Support multiple metric types: Sum, Gauge, Histogram, and Exponential Histogram
- Optionally forward or drop logs after metric extraction
- Send extracted metrics to a separate metrics pipeline via a configured metrics exporter

## Configuration

The processor requires the following configuration:

```yaml
processors:
  logstometricsprocessor:
    # Required: Component ID of the metrics exporter to send extracted metrics to
    metrics_exporter: otlp/metrics
    
    # Optional: Whether to drop logs after processing (default: false)
    drop_logs: false
    
    # Required: List of metric definitions to extract from logs
    logs:
      - name: log.count
        description: "Count of log records"
        sum:
          value: "1"  # Count each log record as 1
```

### Configuration Fields

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `metrics_exporter` | string | Component ID of the metrics exporter to send extracted metrics to | Yes |
| `drop_logs` | bool | Whether to drop logs after processing. If `false`, logs are forwarded to the next consumer. Default: `false` | No |
| `logs` | array | List of metric definitions to extract from logs | Yes |

### Metric Definition

Each metric definition supports the following fields:

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `name` | string | Name of the metric | Yes |
| `description` | string | Description of the metric | No |
| `unit` | string | Unit of the metric | No |
| `attributes` | array | List of attributes to include in the metric | No |
| `include_resource_attributes` | array | List of resource attributes to include in the metric | No |
| `conditions` | array | OTTL conditions (ORed) that must evaluate to true for the metric to be processed | No |
| `sum` | object | Sum metric configuration | One of sum/gauge/histogram/exponential_histogram |
| `gauge` | object | Gauge metric configuration | One of sum/gauge/histogram/exponential_histogram |
| `histogram` | object | Histogram metric configuration | One of sum/gauge/histogram/exponential_histogram |
| `exponential_histogram` | object | Exponential histogram metric configuration | One of sum/gauge/histogram/exponential_histogram |

### Metric Types

#### Sum

```yaml
sum:
  value: "attributes[\"log.duration\"]"  # OTTL expression for the value
```

#### Gauge

```yaml
gauge:
  value: "attributes[\"log.duration\"]"  # OTTL expression for the value
```

#### Histogram

```yaml
histogram:
  value: "attributes[\"log.duration\"]"  # OTTL expression for the value
  count: "1"  # Optional: OTTL expression for the count (default: 1)
  buckets: [1, 10, 50, 100, 200]  # Explicit bucket boundaries
```

#### Exponential Histogram

```yaml
exponential_histogram:
  value: "attributes[\"log.duration\"]"  # OTTL expression for the value
  count: "1"  # Optional: OTTL expression for the count (default: 1)
  max_size: 160  # Maximum number of buckets (default: 160)
```

## Examples

### Basic Configuration: Count Log Records

```yaml
processors:
  logstometricsprocessor:
    metrics_exporter: otlp/metrics
    drop_logs: false
    logs:
      - name: logrecord.count
        description: "Count of log records"
        sum:
          value: "1"
```

### Extract Metrics with Conditions

```yaml
processors:
  logstometricsprocessor:
    metrics_exporter: otlp/metrics
    drop_logs: false
    logs:
      - name: error.log.count
        description: "Count of error logs"
        conditions:
          - severity_text == "ERROR"
        sum:
          value: "1"
      - name: log.duration.histogram
        description: "Log processing duration histogram"
        histogram:
          value: attributes["log.duration"]
          buckets: [1, 10, 50, 100, 200]
```

### Extract Metrics and Drop Logs

```yaml
processors:
  logstometricsprocessor:
    metrics_exporter: otlp/metrics
    drop_logs: true  # Drop logs after extracting metrics
    logs:
      - name: logrecord.count
        description: "Count of log records"
        sum:
          value: "1"
```

### Extract Metrics with Attributes

```yaml
processors:
  logstometricsprocessor:
    metrics_exporter: otlp/metrics
    drop_logs: false
    logs:
      - name: log.count.by.service
        description: "Count logs by service"
        attributes:
          - key: service.name
        sum:
          value: "1"
```

### Extract Gauge Metrics from Log Body

```yaml
processors:
  logstometricsprocessor:
    metrics_exporter: otlp/metrics
    drop_logs: false
    logs:
      - name: memory.usage.mb
        description: "Memory usage from logs"
        gauge:
          value: ExtractGrokPatterns(body, "Memory usage %{NUMBER:memory_mb:int}MB")["memory_mb"]
```

## Pipeline Configuration

The processor must be configured with a metrics exporter that exists in the same collector configuration:

```yaml
exporters:
  otlp/metrics:
    endpoint: localhost:4317
    tls:
      insecure: true

processors:
  logstometricsprocessor:
    metrics_exporter: otlp/metrics
    drop_logs: false
    logs:
      - name: logrecord.count
        description: "Count of log records"
        sum:
          value: "1"

service:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [logstometricsprocessor]
      exporters: [otlp/logs]
    metrics:
      receivers: []
      processors: []
      exporters: [otlp/metrics]  # Metrics extracted by the processor are sent here
```

## Telemetry

The processor reports the following internal metrics:

- `processor_logstometrics_logs_processed`: Number of log records processed
- `processor_logstometrics_metrics_extracted`: Number of metrics extracted from logs
- `processor_logstometrics_errors`: Number of errors encountered during processing
- `processor_logstometrics_logs_dropped`: Number of logs dropped when `drop_logs` is true
- `processor_logstometrics_processing_duration`: Time spent processing logs and extracting metrics (in milliseconds)

## Performance Considerations

- The processor processes logs synchronously and extracts metrics in-memory
- Metric aggregation is performed efficiently using hash-based lookups
- Processing duration is tracked and reported via telemetry metrics
- Errors during metric extraction are logged but do not block log forwarding (unless `drop_logs` is true and metrics export fails)

## Limitations

- Only supports log records (not traces, metrics, or profiles)
- Metrics exporter must be available at processor startup
- Metrics are sent immediately after extraction (no batching)

## See Also

- [OTTL Documentation](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl)
- [Signal to Metrics Connector](../connector/signaltometricsconnector/README.md) - Similar functionality as a connector

