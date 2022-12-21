# Exception Metrics Processor

| Status                   |               |
| ------------------------ |---------------|
| Stability                | [development] |
| Supported pipeline types | traces        |
| Distributions            | [contrib]     |

Extract metrics from collected exceptions from spans.

Each metric will have _at least_ the following dimensions because they are common across all spans:
- Service name
- Span kind
- Status code
- Exception message
- Exception type

This processor lets traces to continue through the pipeline unmodified.

The following settings are required:

- `metrics_exporter`: the name of the exporter that this processor will write metrics to. This exporter **must** be present in a pipeline.

The following settings can be optionally configured:

- `dimensions`: the list of dimensions to add together with the default dimensions defined above.
  
  Each additional dimension is defined with a `name` which is looked up in the span's collection of attributes or
  resource attributes (AKA process tags) such as `ip`, `host.name` or `region`.
  
  If the `name`d attribute is missing in the span, the optional provided `default` is used.
  
  If no `default` is provided, this dimension will be **omitted** from the metric.

## Examples

The following is a simple example usage of the exceptionmetrics processor.

For configuration examples on other use cases, please refer to [More Examples](#more-examples).

The full list of settings exposed for this processor are documented [here](./config.go).

```yaml
receivers:
  jaeger:
    protocols:
      thrift_http:
        endpoint: "0.0.0.0:14278"

  # Dummy receiver that's never used, because a pipeline is required to have one.
  otlp/exceptionmetrics:
    protocols:
      grpc:
        endpoint: "localhost:12345"

  otlp:
    protocols:
      grpc:
        endpoint: "localhost:55677"

processors:
  batch:
  exceptionmetrics:
    metrics_exporter: prometheus  

exporters:
  jaeger:
    endpoint: localhost:14250

  prometheus:
    endpoint: "0.0.0.0:8889"

service:
  pipelines:
    traces:
      receivers: [jaeger]
      processors: [exceptionmetrics, batch]
      exporters: [jaeger]

    metrics:
      receivers: [otlp/exceptionmetrics]
      exporters: [prometheus]
```

### More Examples

For more example configuration covering various other use cases, please visit the [testdata directory](./testdata).

[development]: https://github.com/open-telemetry/opentelemetry-collector#development
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
