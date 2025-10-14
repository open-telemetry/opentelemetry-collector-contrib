# Streaming Aggregation Processor

## Overview

The Streaming Aggregation Processor aggregates metrics over time windows for the OpenTelemetry Collector. It provides automatic type-based aggregation with configurable time windows.

## What It Does

The processor automatically aggregates metrics based on their type:

| Metric Type               | Aggregation Method | Description                     |
| ------------------------- | ------------------ | ------------------------------- |
| **Gauge**                 | Last Value         | Keeps the most recent value     |
| **Counter/Sum**           | Sum                | Adds all values together        |
| **Histogram**             | Bucket Merging     | Combines histogram buckets      |
| **Exponential Histogram** | Scale-Aware Merge  | Merges with scale normalization |
| **Summary**               | Sum & Count        | Combines sum and count values   |

## Key Behavior

### Label Dropping

The processor drops all labels/attributes from metrics, aggregating by metric name only:

```
Input:
  http_requests{endpoint="/api/users", method="GET"} = 100
  http_requests{endpoint="/api/orders", method="POST"} = 50

Output:
  http_requests = 150  # Labels dropped, values aggregated
```

### Temporality Handling

- **Delta Temporality**: Values are directly aggregated
- **Cumulative Temporality**: Processor computes deltas before aggregation

## Configuration

### Basic Usage

```yaml
processors:
  streamingaggregation:
    window_size: 30s # How long to aggregate metrics
    max_memory_mb: 100 # Memory limit in megabytes
```

### Configuration Options

| Parameter              | Type       | Default | Description                         |
| ---------------------- | ---------- | ------- | ----------------------------------- |
| `window_size`          | duration   | `30s`   | Duration of each aggregation window |
| `max_memory_mb`        | int        | `100`   | Maximum memory usage in megabytes   |
| `stale_data_threshold` | duration   | `5m`    | Threshold for detecting stale data  |
| `metrics`              | []object   | `[]`    | Array of metric filtering rules (empty = filter all) |
| `metrics[].match`      | string     | -       | Regex pattern to match metric names |

### Complete Example

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

processors:
  streamingaggregation:
    window_size: 30s
    max_memory_mb: 100
    metrics:
      - match: "^http_.*"     # Process HTTP metrics
      - match: ".*_total$"    # Process counter metrics ending with "_total"

exporters:
  logging:
    loglevel: info

service:
  pipelines:
    metrics:
      receivers: [otlp]
      processors: [streamingaggregation]
      exporters: [logging]
```

## Examples

### Example 1: Application Metrics

**Input metrics:**

```
app.requests{service="api", endpoint="/users"} = 100 (counter)
app.requests{service="api", endpoint="/orders"} = 200 (counter)
app.latency{service="api", endpoint="/users"} = [histogram data]
app.memory{service="api", instance="1"} = 512 (gauge)
```

**Output after 30s window:**

```
app.requests = 300 (sum of all requests)
app.latency = [merged histogram data]
app.memory = 512 (last value seen)
```

### Example 2: Infrastructure Metrics

**Input metrics:**

```
cpu.usage{host="server1"} = 45.2 (gauge)
cpu.usage{host="server2"} = 62.1 (gauge)
network.bytes{host="server1", interface="eth0"} = 1000 (counter)
network.bytes{host="server2", interface="eth0"} = 2000 (counter)
```

**Output after 30s window:**

```
cpu.usage = 62.1 (last value from either host)
network.bytes = 3000 (sum across both hosts)
```

## Use Cases

This processor is useful when you want to:

- **Reduce metric cardinality** by aggregating across all label dimensions
- **Get system-wide views** of metrics across multiple sources
- **Simplify monitoring** by focusing on aggregate values rather than per-instance metrics
- **Reduce storage costs** by storing fewer metric series

## Architecture

The processor uses a double-buffer design where metrics are aggregated in time windows and exported when windows complete.

### File Structure

```
streamingaggregationprocessor/
├── aggregator.go                      # Core aggregation logic
├── processor.go                       # Main processor
├── window.go                          # Time window utilities
├── config.go                          # Configuration
├── factory.go                         # Component factory
├── internal/aggregation/              # Type-specific aggregation
│   ├── counter.go                     # Counter aggregation
│   ├── gauge.go                       # Gauge aggregation
│   ├── histogram.go                   # Histogram aggregation
│   └── ...
└── *_test.go                          # Tests
```

## Monitoring

The processor exports these internal metrics:

- `streamingaggregation_metrics_received_total` - Total metrics received
- `streamingaggregation_metrics_processed_total` - Successfully processed
- `streamingaggregation_memory_usage_bytes` - Current memory usage
- `streamingaggregation_active_series_count` - Number of active series

## Limitations

- **All labels are dropped** - only metric names are preserved
- **Memory usage grows** with the number of unique metric names
- **Window boundaries are fixed** - metrics are aggregated within time windows only
- **No label-based aggregation** - cannot aggregate by specific label values

## When Not to Use

This processor may not be suitable if you:

- Need to preserve specific labels or dimensions
- Require per-instance or per-service metric breakdowns
- Want to aggregate only specific label combinations
- Need real-time (non-windowed) aggregation
