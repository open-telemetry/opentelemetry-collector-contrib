# Streaming Aggregation Processor

## Overview

The Streaming Aggregation Processor is a high-performance, zero-configuration metrics aggregation processor for the OpenTelemetry Collector. It automatically aggregates metrics based on their type using a single-instance, timestamp-based windowing architecture optimized for pre-sharded deployments.

## Key Features

- **Zero Configuration**: Automatic type-based aggregation - no rules needed
- **Single-Instance Design**: Optimized for upstream sharding (e.g., load balancer → stats-relay → shards)
- **Time-Window Aggregation**: Configurable windows with automatic export
- **All Metric Types**: Handles gauges, counters, histograms, exponential histograms, and summaries
- **Memory Management**: Automatic eviction and backpressure handling
- **Production Ready**: Graceful shutdown, statistics reporting, and comprehensive monitoring

## Architecture

The processor uses a single-instance architecture designed for pre-sharded deployments:

```
[Upstream Sharding]
       ↓
[OTEL Collector Instance]
       ↓
[Streaming Aggregation]
       ↓
  [Time Windows]  ← Multiple timestamp-based windows
       ↓
[Aggregated Metrics]
```

**Important**: This processor is designed to work as a single shard in a larger distributed system where upstream components (like a stats-relay or load balancer) handle metric routing. Each OTEL Collector instance with this processor acts as one shard, similar to a StatsD backend.

The processor:

- Maintains multiple time windows for aggregation
- Aggregates metrics automatically by type
- Exports completed windows on schedule
- Handles late-arriving metrics within tolerance

## Automatic Aggregation

The processor automatically aggregates metrics based on their type - no configuration required:

| Metric Type               | Aggregation Method | Description                              |
| ------------------------- | ------------------ | ---------------------------------------- |
| **Gauge**                 | Last Value         | Keeps the most recent value              |
| **Counter/Sum**           | Sum                | Sums all values in the window            |
| **Histogram**             | Merge Buckets      | Combines histogram buckets               |
| **Exponential Histogram** | Merge Buckets      | Scale-aware bucket merging               |
| **Summary**               | Sum & Count        | Merges sum and count (quantiles limited) |

### Temporality Handling

The processor correctly handles both delta and cumulative temporality:

- **Delta Temporality**: Values are directly summed as they represent new data
- **Cumulative Temporality**: The processor computes deltas from cumulative values before aggregation
  - For counters: Tracks last cumulative value and computes delta
  - For histograms: Tracks last cumulative values (sum, count, buckets) and computes deltas
  - Handles counter resets gracefully by treating the new value as the delta

This ensures accurate aggregation regardless of the input temporality, preventing double-counting of cumulative values.

#### Histogram State Preservation

The processor maintains proper cumulative state for histograms across window rotations:

- **Window-specific state**: Reset each window (current window's bucket counts, sum, count)
- **Cumulative tracking state**: Preserved across windows (total accumulated values)
- **Export behavior**: Always exports cumulative totals, ensuring monotonically increasing values

This design prevents the "sawtooth" pattern in aggregated histograms and ensures correct cumulative values are maintained even as windows rotate. This fix also resolves "out of order sample" errors that can occur when exporting to time-series databases like Prometheus, ensuring reliable data delivery without drops.

### Label Dropping for True Streaming Aggregation

**Important**: This processor implements true streaming aggregation by **dropping all labels/attributes** from metrics. This provides:

- **Maximum cardinality reduction**: All data points with the same metric name are aggregated into a single series
- **Consistent behavior**: All metric types (gauges, counters, histograms, etc.) follow the same label-dropping logic
- **Memory efficiency**: Significantly reduces memory usage by eliminating label combinations

For example:

- Input: 5 histogram series with different `endpoint` labels → Output: 1 aggregated histogram
- Input: Multiple counter series with various attributes → Output: 1 aggregated counter

This behavior ensures true streaming aggregation where metrics are combined across all dimensions, providing the highest level of data reduction.

## Configuration

The processor works out of the box with sensible defaults. All configuration is optional:

```yaml
processors:
  streamingaggregation:
    window_size: 30s # Aggregation window size (default: 30s)
    num_windows: 4 # Number of time windows to maintain (default: 4)
    max_memory_mb: 100 # Maximum memory usage in MB (default: 100)
    export_interval: 30s # How often to export (default: 30s)
```

## How It Works

1. **Metrics arrive** at the processor (already sharded by upstream)
2. **Processor** places metrics into appropriate time windows based on timestamp
3. **Each window** aggregates metrics by type:
   - Gauges → Keep last value
   - Counters → Sum values
   - Histograms → Merge buckets
4. **On schedule**, completed windows are exported
5. **Memory management** ensures the processor stays within limits

## Example Scenarios

### Scenario 1: Application Metrics with Label Dropping

Input metrics (multiple series with different labels):

```
app.requests{service="api", endpoint="/users"} = 100 (counter)
app.requests{service="api", endpoint="/products"} = 200 (counter)
app.requests{service="api", endpoint="/orders"} = 150 (counter)
app.latency{service="api", endpoint="/users"} = [histogram data] (histogram)
app.latency{service="api", endpoint="/products"} = [histogram data] (histogram)
app.memory{service="api", instance="1"} = 512 (gauge)
app.memory{service="api", instance="2"} = 480 (gauge)
```

Output after 30s window (labels dropped, metrics aggregated):

```
app.requests = 450 (sum of all requests across all endpoints)
app.latency = [merged histogram across all endpoints]
app.memory = 480 (last value seen)
```

Note: All labels are dropped, resulting in one aggregated series per metric name.

### Scenario 2: System Metrics with Multiple Hosts

Input metrics (from multiple hosts):

```
system.cpu{host="server1"} = 45.2 (gauge)
system.cpu{host="server2"} = 62.1 (gauge)
system.cpu{host="server3"} = 38.8 (gauge)
system.network.bytes{host="server1", interface="eth0"} = 1000 (counter)
system.network.bytes{host="server2", interface="eth0"} = 2000 (counter)
system.network.bytes{host="server3", interface="eth1"} = 1500 (counter)
```

Output after 30s window (all labels dropped):

```
system.cpu = 38.8 (last value from any host)
system.network.bytes = 4500 (sum across all hosts and interfaces)
```

This provides a system-wide view by aggregating metrics across all dimensions.

## Monitoring

The processor exports internal metrics for monitoring:

- `streamingaggregation_metrics_received` - Total metrics received
- `streamingaggregation_metrics_processed` - Successfully processed
- `streamingaggregation_metrics_dropped` - Dropped due to backpressure
- `streamingaggregation_memory_usage_bytes` - Current memory usage
- `streamingaggregation_series_active` - Number of active series
- `streamingaggregation_windows_exported` - Windows successfully exported
- `streamingaggregation_partition_queue_size` - Current queue size per partition
