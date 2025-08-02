# Isolation Forest Processor

Advanced anomaly detection for OpenTelemetry using machine learning.

## Features

- Unsupervised anomaly detection
- Multi-signal support (traces, metrics, logs)
- Real-time processing
- Configurable thresholds

## Configuration

```yaml
processors:
  isolationforest:
    mode: "enrich"
    features:
      traces: ["duration", "error", "http.status_code"]
```

## Status

Alpha - Ready for testing and contribution.
