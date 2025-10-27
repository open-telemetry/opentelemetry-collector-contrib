# Prometheus Metrics Generator (promgen)

A Prometheus metrics generator that creates realistic time-series data for testing and development. The tool serves metrics via HTTP endpoint with support for multiple output formats including standard Prometheus text, JSON, and Protocol Buffers.

## Overview

This generator creates various metric types with realistic data patterns, including mathematical distributions and histogram test cases from the OpenTelemetry collector. It's designed for testing Prometheus scrapers, monitoring systems, and metric processing pipelines.

## Features

- **Multiple Metric Types**: Counter, Gauge, Histogram, Summary, and Native Histogram support
- **Realistic Data Patterns**: Mathematical distributions (gamma, exponential, sinusoidal)
- **Comprehensive Test Cases**: 25+ histogram scenarios from OTEL test suite
- **Multiple Output Formats**: Prometheus text, JSON, and Protocol Buffers
- **Real-time Updates**: Metrics update every second with time-based patterns
- **Deterministic Generation**: Seeded random number generator for reproducible results

## Usage

### Start the Generator

```bash
go run .
```

The server starts on port 8080 and serves metrics at `/metrics`.

### Scrape Metrics

**Standard Prometheus format:**

```bash
curl http://localhost:8080/metrics
```

**JSON format:**

```bash
curl -H "Accept: application/json" http://localhost:8080/metrics
```

**Protocol Buffers format:**

```bash
curl -H "Accept: application/vnd.google.protobuf" http://localhost:8080/metrics -o prom.bin
```

### Inspect Protobuf Metrics

```bash
go run . -read prom.bin
```

## Generated Metrics

### Core Metrics

#### `monotonic_counter`

- **Type**: Counter
- **Pattern**: Continuously increasing by 1 every second
- **Use Case**: Testing counter scraping and rate calculations

**Example Output:**

```text
# HELP monotonic_counter A counter that increases forever
# TYPE monotonic_counter counter
monotonic_counter 42
```

#### `sinusoidal_gauge`

- **Type**: Gauge  
- **Pattern**: Oscillates between -1 and 1 with 20-second period
- **Formula**: `sin(2π * timestamp / 20)`
- **Use Case**: Testing gauge variations and alerting thresholds

**Example Output:**

```text
# HELP sinusoidal_gauge A gauge that oscillates between -1 and 1
# TYPE sinusoidal_gauge gauge
sinusoidal_gauge 0.8090169943749475
```

#### `gamma_histogram`

- **Type**: Histogram
- **Pattern**: Values follow gamma distribution (shape=2.0, scale=2.0)
- **Buckets**: Default Prometheus buckets (.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, +Inf)
- **Updates**: 0-9 random observations per second

**Example Output:**

```text
# HELP gamma_histogram A histogram whose values follow a gamma distribution
# TYPE gamma_histogram histogram
gamma_histogram_bucket{le="0.005"} 0
gamma_histogram_bucket{le="0.01"} 0
gamma_histogram_bucket{le="0.025"} 1
gamma_histogram_bucket{le="0.05"} 3
gamma_histogram_bucket{le="0.1"} 8
gamma_histogram_bucket{le="0.25"} 15
gamma_histogram_bucket{le="0.5"} 25
gamma_histogram_bucket{le="1"} 42
gamma_histogram_bucket{le="2.5"} 68
gamma_histogram_bucket{le="5"} 89
gamma_histogram_bucket{le="10"} 95
gamma_histogram_bucket{le="+Inf"} 100
gamma_histogram_sum 234.56
gamma_histogram_count 100
```

#### `exponential_summary`

- **Type**: Summary
- **Pattern**: Values follow exponential distribution
- **Quantiles**: 50th (±5%), 90th (±1%), 99th (±0.1%)
- **Updates**: 0-9 random observations per second

**Example Output:**

```text
# HELP exponential_summary A summary whose values follow an exponential distribution
# TYPE exponential_summary summary
exponential_summary{quantile="0.5"} 0.693
exponential_summary{quantile="0.9"} 2.303
exponential_summary{quantile="0.99"} 4.605
exponential_summary_sum 156.78
exponential_summary_count 100
```

#### `gamma_native_histogram`

- **Type**: Native Histogram
- **Pattern**: Values follow gamma distribution with native histogram buckets
- **Configuration**: Bucket factor 1.1, zero threshold 1e-6
- **Updates**: 0-9 random observations per second

Native histogram data is only available in protobuf format. See [Native Histograms [EXPERIMENTAL]](https://prometheus.io/docs/specs/native_histograms/#exposition-formats).

### OTEL Test Case Histograms

The generator includes 25+ histogram test cases with realistic scenarios:

#### `tc_basic_histogram`

- **Scenario**: Standard payment service latency
- **Buckets**: [25, 50, 75, 100, 150]ms
- **Attributes**: `service.name="payment-service"`
- **Pattern**: 101 samples, realistic distribution

#### `tc_tail_heavy_histogram` 

- **Scenario**: Heavy tail distribution with outliers
- **Pattern**: Most values concentrated in upper buckets
- **Use Case**: Testing percentile calculations with skewed data

#### `tc_many_buckets`

- **Scenario**: Detailed metrics with 21 buckets
- **Range**: 1ms to 1000ms with fine granularity
- **Use Case**: Testing high-cardinality histogram processing

#### `tc_large_numbers`

- **Scenario**: Batch processing with values up to 1 billion
- **Use Case**: Testing numeric precision and scaling

#### `tc_very_small_numbers`

- **Scenario**: Microsecond-precision timing
- **Range**: 0.00000001 to 0.000006 seconds
- **Use Case**: Testing floating-point precision

#### Bucket Scale Test Cases

- **`tc_126_buckets`**: 126 buckets for testing request splitting
- **`tc_176_buckets`**: 176 buckets for EMF path testing  
- **`tc_225_buckets`**: 225 buckets for multi-request scenarios
- **`tc_325_buckets`**: 325 buckets for maximum scale testing

## What Customers Can Expect

### Scraping Behavior

- **Update Frequency**: Metrics update every second
- **Deterministic Values**: Same seed produces identical sequences
- **Realistic Patterns**: Mathematical distributions mirror real-world data
- **Stable Cardinality**: Fixed set of metrics and labels

### Performance Characteristics

- **Response Time**: Sub-millisecond for standard scrapes
- **Memory Usage**: Stable, no metric explosion
- **CPU Usage**: Minimal, optimized update loops
- **Concurrent Scrapes**: Thread-safe, supports multiple scrapers

### Data Patterns

- **Counters**: Monotonically increasing, never reset
- **Gauges**: Smooth oscillations, predictable ranges
- **Histograms**: Realistic bucket distributions
- **Summaries**: Accurate quantile calculations

### Format Support

- **Prometheus Text**: Standard exposition format
- **JSON**: Structured data for programmatic access
- **Protocol Buffers**: Efficient binary format for high-volume scenarios

## Development

### Adding Custom Metrics

```go
customMetric := MetricDefinition{
    Name: "custom_metric",
    Type: TypeGauge,
    Help: "My custom metric",
    Update: func(collector prometheus.Collector, timestamp time.Time) error {
        gauge := collector.(*prometheus.GaugeVec)
        metric, _ := gauge.GetMetricWith(prometheus.Labels{})
        metric.Set(float64(timestamp.Unix()))
        return nil
    },
}
generator.AddMetric(customMetric)
```

### Custom Collectors

```go
customCollector := MetricDefinition{
    Name: "custom_histogram",
    Type: TypeHistogram,
    CreateCollector: func() (prometheus.Collector, error) {
        return prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "custom_histogram",
                Help:    "Custom bucket boundaries",
                Buckets: []float64{1, 5, 10, 50, 100},
            },
            []string{"label1", "label2"},
        ), nil
    },
}
```

## Use Cases

- **Prometheus Testing**: Validate scraper configurations and storage
- **Monitoring Development**: Test dashboards and alerting rules  
- **Performance Testing**: Load test metric ingestion pipelines
- **Integration Testing**: End-to-end monitoring system validation
- **Training**: Learn Prometheus metric types and patterns
