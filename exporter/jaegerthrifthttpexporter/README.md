# Jaeger Thrift Exporter

This exporter supports sending trace data to [Jaeger](https://www.jaegertracing.io) over Thrift HTTP.

*WARNING:* The Jaeger exporter is the recommended approach for exporting traces from an OpenTelemetry collector to Jaeger. This Jaeger Thrift exporter should only be used to export traces to a Jaeger collector that is unable to expose the [gRPC API](https://www.jaegertracing.io/docs/1.27/apis/#protobuf-via-grpc-stable).

Supported pipeline types: traces

## Configuration

The following settings are required:

- `url` (no default): target to which the exporter is going to send Jaeger trace data,
using the Thrift HTTP protocol.

The following settings can be optionally configured:

- `timeout` (default = 5s): the maximum time to wait for a HTTP request to complete
- `headers` (no default): headers to be added to the HTTP request

Example:

```yaml
exporters:
  jaeger_thrift:
    url: "http://some.other.location/api/traces"
    timeout: 2s
    headers:
      added-entry: "added value"
      dot.test: test
```

The full list of settings exposed for this exporter are documented [here](config.go)
with detailed sample configurations [here](testdata/config.yaml).

This exporter also offers proxy support as documented
[here](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter#proxy-support).
