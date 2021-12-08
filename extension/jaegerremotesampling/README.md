# Jaeger's Remote Sampling extension

This extension allows serving sampling strategies following the Jaeger's remote sampling API. This extension can be configured to proxy requests to a backing remote sampling server, which could potentially be a Jaeger Collector down the pipeline, or a static JSON file from the local file system.

Although this extension is derived from Jaeger, it can be used by any clients who can consume this standard, such as the [OpenTelemetry Java SDK](https://github.com/open-telemetry/opentelemetry-java/tree/v1.9.1/sdk-extensions/jaeger-remote-sampler).

## Configuration

```yaml
extensions:
  jaegerremotesampling:
    remote:
      endpoint: jaeger-collector:14250
  jaegerremotesampling/1:
    strategy_file: /etc/otel/sampling_strategies.json
```

