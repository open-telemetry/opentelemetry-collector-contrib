## `severity_parser` operator

The `severity_parser` operator sets the severity on an entry by parsing a value from the body.

### Configuration Fields

| Field            | Default           | Description |
| ---              | ---               | ---         |
| `id`             | `severity_parser` | A unique identifier for the operator. |
| `output`         | Next in pipeline  | The `id` for the operator to send parsed entries to. |
| `parse_from`     | required          | The [field](../types/field.md) from which the value will be parsed. |
| `on_error`       | `send`            | The behavior of the operator if it encounters an error. See [on_error](../types/on_error.md). |
| `preset`         | `default`         | A predefined set of values that should be interpreted at specific severity levels. |
| `mapping`        |                   | A formatted set of values that should be interpreted as severity levels. |
| `overwrite_text` | `false`           | If `true`, the severity text will be set to the [standard short name](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/logs/data-model.md#displaying-severity) corresponding to the severity number. |
| `if`             |                   | An [expression](../types/expression.md) that, when set, will be evaluated to determine whether this operator should be used for the given entry. This allows you to do easy conditional parsing without branching logic with routers. |

### Example Configurations

Several detailed examples are available [here](../types/severity.md).
