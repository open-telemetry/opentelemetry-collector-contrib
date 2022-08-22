# Mezmo Exporter

This exporter supports sending OpenTelemetry log data to
[Mezmo](https://mezmo.com).

# Configuration options:

- `ingest_url` (optional): Specifies the URL to send ingested logs to.  If not 
specified, will default to `https://logs.mezmo.com/otel/ingest/rest`.
- `ingest_key` (required): Ingestion key used to send log data to Mezmo.  See
[Ingestion Keys](https://docs.mezmo.com/docs/ingestion-key) for more details.

# Example:
## Simple Log Data

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: ":4317"

processors:
  resourcedetection:
    detectors:
      - system
    system:
      hostname_sources:
        - os

exporters:
  mezmo:
    ingest_url: "https://logs.mezmo.com/otel/ingest/rest"
    ingest_key: "00000000000000000000000000000000"

service:
  pipelines:
    logs:
      receivers: [ otlp ]
      processors: [ resourcedetection ]
      exporters: [ mezmo ]
```