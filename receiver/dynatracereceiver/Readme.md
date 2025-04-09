
# Dynatrace Receiver for OpenTelemetry Collector

This is a custom OpenTelemetry receiver that pulls metrics from the Dynatrace Metrics API v2.

Its main goal is to fetch metrics from an existing Dynatrace setup and make them available to your OpenTelemetry pipeline. Whether you want to export them, log them, or use them in a dashboard. You do need to setup your own .env with the given variables and also adjust the config.yaml depending on your needed metrics. 

---

## Motivation

The goal is to use Dynatrace for performance monitoring and bring all their data into one central pipeline using OpenTelemetry.

This receiver does this by:

- Automatically fetching metrics from Dynatrace on a schedule  
- Transforming them into OpenTelemetry metrics  
- Allowing you to add custom labels (like environment or system ID)  
- Sending them to any OpenTelemetry exporter (Kafka, Prometheus, OTLP, etc.)

---

## Features

- Pulls metrics from Dynatrace Metrics API v2  
- Fully configurable via `config.yaml`  
- Compatible with any OpenTelemetry pipeline (processors/exporters)

---

## Example Configuration (`config.yaml`)

```yaml
receivers:
  dynatrace:
    API_ENDPOINT: ${env:API_ENDPOINT}
    API_TOKEN: ${env:API_TOKEN}
    metric_selectors:
      - builtin:containers.cpu.usageTime
      - builtin:containers.memory.residentSetBytes
    resolution: 1h
    from: "2025-04-01T00:00:00Z"
    to: "2025-04-03T00:00:00Z"
    poll_interval: 30s
    max_retries: 3
    http_timeout: 30s

#just examples
processors:
  batch:
  resource:
    attributes:
      - key: environment
        value: ${env:DEPLOYMENT_ENVIRONMENT}
        action: upsert
      - key: team_owner
        value: "team-sid"
        action: insert

# add your custom or other exporters to push the data
exporters:
  logging:
    verbosity: detailed

service:
  pipelines:
    metrics:
      receivers: [dynatrace]
      processors: [batch, resource]
      exporters: [logging]
```

You can use `custom_labels` to tag metrics with helpful context like environment (`prod`, `stage`) or system identifiers.
The idea is to make it as simple as possible and adjust needs with the config.yaml

---

