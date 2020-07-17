# StatsD Receiver

StatsD/DogStatsD receiver.

## Sample Configuration

```yaml
receivers:
  statsd:
    # endpoint: "localhost:8125"

    # timeout: "50s"

exporters:
  logging:

service:
  pipelines:
    metrics:
     receivers: [statsd]
     exporters: [logging]
```