# Mezmo Exporter

This exporter supports sending OpenTelemetry log data to [Mezmo](https://mezmo.com).

# Configuration options:

- `endpoint` (required): LogService's [Region](https://cloud.tencent.com/document/product/614/56473).
- `ingestion_key` (required): LogService's LogSet ID.

# Example:
## Simple Log Data

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: ":4317"

exporters:
  mezmo:
    endpoint = "https://logdna.com/log/ingest"
    ingestion_key = "00000000000000000000000000000000"

service:
  pipelines:
    logs:
      receivers: [ otlp ]
      exporters: [ mezmo ]
```