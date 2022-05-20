# Mezmo Exporter

This exporter supports sending OpenTelemetry log data to [LogDNA (Mezmo)](https://logdna.com).

# Configuration options:

- `ingest_url` (optional): Specifies the URL to send ingested logs to.  If not specified, will default to `https://logs.logdna.com/log/ingest`.
- `ingest_key` (required): Ingestion key used to send log data to LogDNA.  See [Ingestion Keys](https://docs.logdna.com/docs/ingestion-key) for more details.

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
    ingest_url: "https://logs.logdna.com/log/ingest"
    ingest_key: "00000000000000000000000000000000"

service:
  pipelines:
    logs:
      receivers: [ otlp ]
      exporters: [ mezmo ]
```