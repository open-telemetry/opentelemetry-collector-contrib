# Jaeger's Remote Sampling extension

This extension allows serving sampling strategies following the Jaeger's remote sampling API. This extension can be configured to proxy requests to a backing remote sampling server, which could potentially be a Jaeger Collector down the pipeline, or a static JSON file from the local file system.

By default, two listeners are made available:
- `localhost:5778`, following the legacy remote sampling endpoint as defined by Jaeger
- `localhost:14250`, following the gRPC remote sampling endpoint, also defined by Jaeger

Note that the port `14250` will clash with the Jaeger Receiver. When both are used, it's recommended to change this extension to use another port.

Although this extension is derived from Jaeger, it can be used by any clients who can consume this standard, such as the [OpenTelemetry Java SDK](https://github.com/open-telemetry/opentelemetry-java/tree/v1.9.1/sdk-extensions/jaeger-remote-sampler).

## Configuration

```yaml
extensions:
  jaegerremotesampling:
    remote:
      endpoint: jaeger-collector:14250
  jaegerremotesampling/1:
    strategy_file: /etc/otel/sampling_strategies.json
    grpc:
      endpoint: :15251
    http:
      endpoint: :5878
```

A sampling strategy file could look like:

```json
{
  "service_strategies": [
    {
      "service": "foo",
      "type": "probabilistic",
      "param": 0.8,
      "operation_strategies": [
        {
          "operation": "op1",
          "type": "probabilistic",
          "param": 0.2
        },
        {
          "operation": "op2",
          "type": "probabilistic",
          "param": 0.4
        }
      ]
    },
    {
      "service": "bar",
      "type": "ratelimiting",
      "param": 5
    }
  ],
  "default_strategy": {
    "type": "probabilistic",
    "param": 0.5,
    "operation_strategies": [
      {
        "operation": "/health",
        "type": "probabilistic",
        "param": 0.0
      },
      {
        "operation": "/metrics",
        "type": "probabilistic",
        "param": 0.0
      }
    ]
  }
}
```
Source: https://www.jaegertracing.io/docs/1.28/sampling/#collector-sampling-configuration