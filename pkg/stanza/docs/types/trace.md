## Trace Parsing

Traces context fields are defined in the [OpenTelemetry Logs Data Model](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/logs/data-model.md#trace-context-fields).


### `trace` parsing parameters

Parser operators can parse a trace context and attach the resulting values to a log entry.

| Field                    | Default       | Description |
| ---                      | ---           | ---         |
| `trace_id.parse_from`    | `trace_id`    | A [field](../types/field.md) that indicates the field to be parsed as a trace ID. |
| `span_id.parse_from`     | `span_id`     | A [field](../types/field.md) that indicates the field to be parsed as a span ID. |
| `trace_flags.parse_from` | `trace_flags` | A [field](../types/field.md) that indicates the field to be parsed as trace flags. |


### How to use trace parsing

All parser operators, such as [`regex_parser`](../operators/regex_parser.md) support these fields inside of a `trace` block.

If a `trace` block is specified, the parser operator will perform the trace parsing _after_ performing its other parsing actions, but _before_ passing the entry to the specified output operator.

```yaml
- type: regex_parser
  regex: '^TraceID=(?P<trace_id>\S*) SpanID=(?P<span_id>\S*) TraceFlags=(?P<trace_flags>\d*)'
  trace:
    trace_id:
      parse_from: attributes.trace_id
    span_id:
      parse_from: attributes.span_id
    trace_flags:
      parse_from: attributes.trace_flags
```

---

As a special case, the [`trace_parser`](../operators/trace_parser.md) operator supports these fields inline. This is because trace parsing is the primary purpose of the operator.

```yaml
- type: trace_parser
  trace_id:
    parse_from: attributes.trace_id
  span_id:
    parse_from: attributes.span_id
  trace_flags:
    parse_from: attributes.trace_flags
```
