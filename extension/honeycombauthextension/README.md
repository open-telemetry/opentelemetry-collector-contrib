# Honeycomb Authenticator

This extension provides Honeycomb authentication for making requests
to [Honeycomb OpenTelemetry API](https://docs.honeycomb.io/getting-data-in/opentelemetry-overview/#using-the-honeycomb-opentelemetry-endpoint)
.

The authenticator type has to be set to `honeycombauth`.

## Configuration

The following settings are required:

- `team` (default = `$HONEYCOMB_TEAM`): set the value to your Honeycomb write key.
- `dataset` (default = `$HONEYCOMB_DATASET`): set to the name of the target dataset.

```yaml
extensions:
  honeycombauth:
    # or set `HONEYCOMB_TEAM` environment variable
    team: "my-team"
    # or set `HONEYCOMB_DATASET` environment variable
    dataset: "my-dataset"

receivers:
  otlp:
    protocols:
      grpc:

exporters:
  otlp:
    endpoint: api.honeycomb.io:443
    auth:
      authenticator: honeycombauth

service:
  extensions:
    - honeycombauth
  pipelines:
    metrics:
      receivers:
        - otlp
      processors: [ ]
      exporters:
        - otlp
```
