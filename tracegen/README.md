# Trace generator for OpenTelemetry

This utility simulates a client generating traces, useful for testing and demonstration purposes.

## Installing

To install the latest version run the following command:

```console
$ go install github.com/open-telemetry/opentelemetry-collector-contrib/tracegen@latest
```

Check the [`go install` reference](https://go.dev/ref/mod#go-install) to install specific versions.

## Running

First, you'll need an OpenTelemetry Collector to receive the trace data. Follow the project's instructions for a detailed setting up guide. The following configuration file should be sufficient:

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: localhost:4317

processors:

exporters:
  logging:

service:
  pipelines:
    traces:
      receivers:
      - otlp
      processors: []
      exporters:
      - logging
```

Once the OpenTelemetry Collector instance is up and running, run `tracegen`:

```console
$ tracegen -otlp-insecure -duration 5s
```

Or, to generate a specific number of traces:
```console
$ tracegen -otlp-insecure -traces 1
```

Check `-help` for all the options.
