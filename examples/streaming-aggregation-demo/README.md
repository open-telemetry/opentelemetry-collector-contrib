# Streaming Aggregation Processor Demo

This demo showcases the custom **Streaming Aggregation Processor** for OpenTelemetry Collector, demonstrating how it aggregates high-frequency metrics into time-based windows for efficient storage and analysis.

## Overview

The demo runs two parallel pipelines:

1. **Raw Pipeline**: Metrics emitted every second without aggregation
2. **Aggregated Pipeline**: Same metrics aggregated into 30-second windows using the streaming aggregation processor

This allows you to visually compare the difference between raw and aggregated metrics in Grafana.

## Architecture

```
┌─────────────────┐
│ Metric Generator│
│   (1 sec rate)  │
└────────┬────────┘
         │
    ┌────┴────┐
    │         │
    ▼         ▼
┌──────────┐ ┌──────────────────┐
│   Raw    │ │    Aggregated    │
│Collector │ │   Collector      │
│(no proc) │ │(streaming agg)   │
└────┬─────┘ └────────┬─────────┘
     │                 │
     ▼                 ▼
┌─────────────────────────────┐
│       Prometheus            │
└──────────────┬──────────────┘
               │
               ▼
┌─────────────────────────────┐
│         Grafana             │
│   (Comparison Dashboard)    │
└─────────────────────────────┘
```

## Features Demonstrated

### Metric Types

- **Gauge**: Temperature readings (last value in window)
- **Counter**: HTTP request counts (sum over window)
- **Histogram**: Response time distributions (merged buckets)
- **UpDownCounter**: Active connections (net change)

### Aggregation Benefits

- **Data Reduction**: ~30x fewer data points
- **Smoother Trends**: Reduced noise in metrics
- **Lower Storage**: Significantly reduced cardinality
- **Consistent Windows**: 30-second time partitions

## Prerequisites

- Docker and Docker Compose
- Go 1.21+ (for building the custom collector)
- 4GB+ available RAM
- Ports 3000, 9090, 4317-4318, 8887-8890 available

## Quick Start

### 1. Build the Custom Collector

First, build the custom OpenTelemetry Collector with the streaming aggregation processor:

```bash
cd collector-builder
chmod +x build.sh
./build.sh
```

This creates a custom collector binary at `./bin/otelcol-streaming`.

### 2. Start the Demo

```bash
# From the streaming-aggregation-demo directory
docker-compose up -d
```

This will:

- Build and start two collector instances (raw and aggregated)
- Start the metric generator
- Launch Prometheus to scrape metrics
- Start Grafana with a pre-configured dashboard

### 3. View the Dashboard

1. Open Grafana: http://localhost:3000
2. Login with `admin/admin` (or skip login as viewer)
3. Navigate to Dashboards → Streaming Aggregation Processor Demo

## Understanding the Dashboard

### Panel Layout

#### Row 1: Temperature Comparison

- **Left**: Raw temperature readings (1-second resolution, noisy)
- **Right**: Aggregated temperature (30-second windows, smooth)

#### Row 2: Request Rate Comparison

- **Left**: Raw request rate per second (high variation)
- **Right**: Aggregated request rate (stable trends)

#### Row 3: Response Time Percentiles

- **Left**: Raw P95/P99 latencies (fluctuating)
- **Right**: Aggregated P95/P99 (consistent values)

#### Row 4: Statistics

- Series count comparison
- Data reduction percentage
- Memory usage metrics

## Configuration

### Streaming Aggregation Processor Settings

Edit `otel-config.yaml` to adjust:

```yaml
processors:
  streamingaggregation:
    window_size: 30s # Size of aggregation windows
    num_windows: 4 # Number of windows to maintain
    max_memory_mb: 100 # Memory limit
    export_interval: 30s # How often to export
```

### Metric Generation Rate

The metric generator emits:

- 5-15 requests per second
- Temperature updates every second
- Connection changes ±5 per second

## Monitoring

### Prometheus Metrics

- View raw Prometheus data: http://localhost:9090
- Query examples:
  - Raw metrics: `raw_temperature_celsius`
  - Aggregated metrics: `aggregated_temperature_celsius`

### Collector Metrics

- Raw collector: http://localhost:8887/metrics
- Aggregated collector: http://localhost:8888/metrics

## Troubleshooting

### Container Issues

```bash
# Check container status
docker-compose ps

# View logs
docker-compose logs collector-aggregated
docker-compose logs metric-generator

# Restart services
docker-compose restart
```

### Build Issues

```bash
# Clean and rebuild
docker-compose down -v
rm -rf collector-builder/bin
./collector-builder/build.sh
docker-compose build --no-cache
docker-compose up -d
```

### Port Conflicts

If ports are already in use, modify `docker-compose.yml`:

- Grafana: Change `3000:3000` to `3001:3000`
- Prometheus: Change `9090:9090` to `9091:9090`

## How It Works

### Time-Based Windows

The processor maintains 4 time windows (default):

```
[Window 0] [Window 1] [Window 2] [Window 3]
 (oldest)                         (current)
```

Every 30 seconds:

1. Windows rotate left
2. Oldest window is exported
3. New window is created
4. Metrics are aggregated within their time window

### Aggregation Strategies

- **Gauges**: Keep last value in window
- **Counters**: Sum all values in window
- **Histograms**: Merge all buckets in window
- **Summaries**: Combine statistics

### Late Arrival Handling

- Metrics can arrive up to 5 seconds late
- Multiple windows handle out-of-order data
- Old metrics beyond tolerance are dropped

## Cleanup

```bash
# Stop all containers
docker-compose down

# Remove volumes (Prometheus/Grafana data)
docker-compose down -v

# Remove built binaries
rm -rf collector-builder/bin
```

## Advanced Usage

### Custom Metrics

Modify `metric-generator/main.go` to add new metric types or change emission patterns.

### Different Window Sizes

Experiment with different aggregation windows:

- 10s for near real-time
- 60s for trending
- 5m for long-term analysis

### Production Considerations

- Adjust `max_memory_mb` based on cardinality
- Use `export_interval` matching your storage resolution
- Configure `num_windows` based on data arrival patterns

## Architecture Details

### Processor Implementation

- Location: `../../processor/streamingaggregationprocessor/`
- Key files:
  - `processor.go`: Main processing logic
  - `window.go`: Time window management
  - `config.go`: Configuration structure

### Memory Management

- Automatic eviction when memory limit exceeded
- LRU eviction of least-used series
- Configurable memory limits per window

## Contributing

To modify the streaming aggregation processor:

1. Edit files in `../../processor/streamingaggregationprocessor/`
2. Rebuild the collector: `./collector-builder/build.sh`
3. Restart containers: `docker-compose restart`

## License

This demo is part of the OpenTelemetry Collector Contrib project and follows the same Apache 2.0 license.
