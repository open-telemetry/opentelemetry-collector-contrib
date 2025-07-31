# Isolation Forest Processor

The Isolation Forest processor applies anomaly detection to metrics using the Isolation Forest algorithm.

## Configuration

```yaml
processors:
  isolationforest:
    num_trees: 100
    subsample_size: 256
    window_size: 1000
    anomaly_threshold: 0.6
    training_interval: 5m
    add_anomaly_score: true
    drop_anomalous_metrics: false
```

For more configuration options, see [config.go](./config.go).

## Features

- Real-time anomaly detection on metrics
- Configurable anomaly thresholds
- Sliding window for continuous learning
- Support for multiple metric types (gauge, sum, histogram, summary)
- Optional anomaly score attribution
- Configurable metric filtering

## Usage

Add the processor to your collector configuration:

```yaml
service:
  pipelines:
    metrics:
      receivers: [otlp]
      processors: [isolationforest, batch]
      exporters: [otlp]
```
