# Log to Span exporter

This exporter receives logs, converts them to trace spans, then exports the spans via OTLP over gRPC.

Supported pipeline types: logs

> :construction: This exporter is alpha. Expect configuration fields to change and expect this exporter to be superseded
> by either a log to span 'translator' ( see https://github.com/open-telemetry/opentelemetry-collector/issues/2336 ) 
> and/or via 'Telemetry Schemas' ( see https://github.com/open-telemetry/oteps/pull/152 ).

**!! IMPORTANT: Both the log originator and all downstream services must use the same Network Time Protocol (NTP)
settings to ensure spans times line up correctly. !!**

## Use Cases

- An upstream system such as a load-balancer or proxy can participate in tracing, lacks the native capabilities to
  record spans, but has the ability to output a log containing the necessary span fields when responding to a request.

## Getting Started

The following configuration options are supported:

- `endpoint` (required, no default): The OTLP gRPC endpoint (host:port) for sending spans to (e.g. otel-collector:4317). The valid syntax
  is described [here](https://github.com/grpc/grpc/blob/master/doc/naming.md).

- `time_format` (required, no default): The time format of `field_map.span_start_time` and `field_map.span_end_time`. Currently,
  'unix_epoch_micro' and 'unix_epoch_nano' are supported.

- `trace_type` (optional, default = `w3c`): The type of trace that the log represents. Currently, only 'w3c' is supported.

- `field_map` (required): Defines the mapping of log attributes to span fields.
  - `span_name` (required, no default): The name of the attribute containing the span name.
  - `span_start_time` (required, no default): The name of the attribute containing the span start time.
  - `span_end_time` (required, no default): The name of the attribute containing the span end time.
  - `span_kind` (optional): The name of the attribute containing the kind of span this log represents.
    Valid attribute values include 'server', 'client', 'consumer', 'producer', and 'internal'. These are directly mapped to
    https://pkg.go.dev/go.opentelemetry.io/collector/consumer/pdata@v0.24.0#SpanKindUNSPECIFIED. Span defaults to 'server'
    when either a 'span_kind' attribute is not found on the log record, or the value is not one of the valid attribute values
    listed previously.
  - `w3c` (optional): The W3C Trace Context field mappings when 'trace_type' is set to 'w3c'.
    - `traceparent` (optional, default = `traceparent`): The name of the attribute containing the W3C Trace Context 'traceparent' header.
    - `tracestate` (optional, default = `tracestate`): The name of the attribute containing the W3C Trace Context 'tracestate' header.
  - `ignored` (optional, no default): A list of attributes to not add to the outbound span.

By default, TLS is enabled:

- `insecure` (optional, default = `false`): whether to enable client transport security for
  the exporter's connection.

As a result, the following parameters are also required:

- `cert_file` (no default): path to the TLS cert to use for TLS required connections. Should
  only be used if `insecure` is set to false.
- `key_file` (no default): path to the TLS key to use for TLS required connections. Should
  only be used if `insecure` is set to false.

Example:

```yaml
exporters:
  logtospan:
    endpoint: "otelcol:4317"
    insecure: false
    ca_file: server.crt
    cert_file: client.crt
    key_file: client.key
    trace_type: "w3c"
    time_format: "unix_epoch_micro"
    field_map:
      w3c:
        traceparent: "traceparent"
        tracestate: "tracestate"
      span_name: "span_name"
      span_start_time: "req_start_time"
      span_end_time: "res_start_time"
      span_kind: "span_kind"
      ignored:
        - "fluent.tag"
```