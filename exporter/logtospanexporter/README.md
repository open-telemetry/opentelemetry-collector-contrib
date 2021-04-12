# Log to Span exporter

This exporter receives logs, converts them to trace spans, then exports the spans via OTLP over gRPC.

Supported pipeline types: logs

> :construction: This exporter is alpha. Expect configuration fields to change.

**!! IMPORTANT: Both the log originator and all downstream services must use the same Network Time Protocol (NTP)
settings to ensure spans times line up correctly. !!**

## Use Cases

- An upstream system such as a load-balancer or proxy can participate in tracing, lacks the native capabilities to
  record spans, but has the ability to output a log containing the necessary span fields when responding to a request.

## Getting Started

The following settings are required:

- `endpoint` (no default): The OTLP gRPC endpoint (host:port) for sending spans to (e.g. otel-collector:4317). The valid syntax
  is described [here](https://github.com/grpc/grpc/blob/master/doc/naming.md).

- `time_format` (no default): The time format of `field_map.span_start_time` and `field_map.span_end_time`. Currently,
  'unix_epoch_micro' and 'unix_epoch_nano' are supported.

- `field_map.span_name` (no default): The name of the attribute containing the span name.
- `field_map.span_start_time` (no default): The name of the attribute containing the span start time.
- `field_map.span_end_time` (no default): The name of the attribute containing the span end time.

The following settings can be optionally configured:

- `trace_type` (default = `w3c`): The type of span that the log represents.

- `field_map.w3c.traceparent` (default = `traceparent`): The name of the attribute containing the W3C Trace
  Context 'traceparent' header. Used when `trace_type` is set to 'w3c'.
- `field_map.w3c.tracestate` (default = `tracestate`): The name of the attribute containing the W3C Trace
  Context 'tracestate' header. Used when `trace_type` is set to 'w3c'.
- `field_map.ignored` (no default): A list of attributes to not add to the outbound span.

By default, TLS is enabled:

- `insecure` (default = `false`): whether to enable client transport security for
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
    time_format: 'unix_epoch_micro'
    field_map:
      w3c:
        traceparent: "traceparent"
        tracestate: "tracestate"
      span_name: "span_name"
      span_start_time: "req_start_time"
      span_end_time: "res_start_time"
      ignored:
        - "fluent.tag"
```