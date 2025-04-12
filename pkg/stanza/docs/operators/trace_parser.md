## `trace_parser` operator

The `trace_parser` operator sets the trace on an entry by parsing a value from the body.

### Configuration Fields

| Field                    | Default          | Description |
| ---                      | ---              | ---         |
| `id`                     | `trace_parser`   | A unique identifier for the operator. |
| `output`                 | Next in pipeline | The `id` for the operator to send parsed entries to. |
| `trace_id.parse_from`    | `trace_id`       | A [field](../types/field.md) that indicates the field to be parsed as a trace ID. |
| `span_id.parse_from`     | `span_id`        | A [field](../types/field.md) that indicates the field to be parsed as a span ID. |
| `trace_flags.parse_from` | `trace_flags`    | A [field](../types/field.md) that indicates the field to be parsed as trace flags. |
| `on_error`               | `send`           | The behavior of the operator if it encounters an error. See [on_error](../types/on_error.md). |


### Example Configurations

Several detailed examples are available [here](../types/trace.md).
