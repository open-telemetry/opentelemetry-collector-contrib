# Jaeger Thrift Exporter

| Status                   |           |
| ------------------------ |-----------|
| Stability                | [beta]    |
| Supported pipeline types | traces    |
| Distributions            | [contrib] |

This exporter supports sending trace data to [Jaeger](https://www.jaegertracing.io) over Thrift HTTP.

*WARNING:* The [Jaeger gRPC Exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/jaegerexporter) is the recommended one for exporting traces from an OpenTelemetry Collector to Jaeger. This Jaeger Thrift Exporter should only be used to export traces to a Jaeger Collector that is unable to expose the [gRPC API](https://www.jaegertracing.io/docs/1.27/apis/#protobuf-via-grpc-stable).

## Configuration

The following settings are required:

- `endpoint` (no default): target to which the exporter is going to send Jaeger trace data,
using the Thrift HTTP protocol.

The following settings can be optionally configured:

- `timeout` (default = 5s): the maximum time to wait for a HTTP request to complete
- `headers` (no default): headers to be added to the HTTP request

Example:

```yaml
exporters:
  jaeger_thrift:
    endpoint: "http://jaeger.example.com/api/traces"
    timeout: 2s
    headers:
      added-entry: "added value"
      dot.test: test
```

The full list of settings exposed for this exporter are documented [here](config.go)
with detailed sample configurations [here](testdata/config.yaml).

This exporter also offers proxy support as documented
[here](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter#proxy-support).

[beta]:https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
