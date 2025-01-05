## `scope_name_parser` operator

The `scope_name_parser` operator sets the scope name on an entry by parsing a value from the specified [field](../types/field.md).

### Configuration Fields

| Field                    | Default             | Description |
| ---                      | ---                 | ---         |
| `id`                     | `scope_name_parser` | A unique identifier for the operator. |
| `output`                 | Next in pipeline    | The `id` for the operator to send parsed entries to. |
| `parse_from`             | `body`              | A [field](../types/field.md) that indicates the field to be parsed as the scope name. |
| `on_error`               | `send`              | The behavior of the operator if it encounters an error. See [on_error](../types/on_error.md). |


### Example Configurations

[Detailed configuration examples are available](../types/scope_name.md).
