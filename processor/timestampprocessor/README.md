# Timestamp Processor for OpenTelemetry Collector

Supported pipeline types: metrics

The timestamp processor will round all timestamps in metrics streams to the nearest <duration>.

Examples:

```yaml
processors:
  timestamp:
    round_to_nearest: 1s
```
