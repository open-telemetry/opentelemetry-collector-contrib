## `time_parser` operator

The `time_parser` operator sets the timestamp on an entry by parsing a value from the body.

### Configuration Fields

| Field         | Default    | Description                                                                                                                                                                                                                              |
| ---           | ---        | ---                                                                                                                                                                                                                                      |
| `id`          | required   | A unique identifier for the operator                                                                                                                                                                                                     |
| `output`      | required   | The connected operator(s) that will receive all outbound entries                                                                                                                                                                         |
| `parse_from`  | required   | A [field](/docs/types/field.md) that indicates the field to be parsed as JSON                                                                                                                                                            |
| `layout_type` | `strptime` | The type of timestamp. Valid values are `strptime`, `gotime`, and `epoch`                                                                                                                                                                |
| `layout`      | required   | The exact layout of the timestamp to be parsed                                                                                                                                                                                           |
| `if`          |            | An [expression](/docs/types/expression.md) that, when set, will be evaluated to determine whether this operator should be used for the given entry. This allows you to do easy conditional parsing without branching logic with routers. |
| `preserve_to` |            | Preserves the unparsed value at the specified [field](/docs/types/field.md)                                                                                                                                                              |
| `on_error`    | `send`     | The behavior of the operator if it encounters an error. See [on_error](/docs/types/on_error.md)                                                                                                                                          |


### Example Configurations

Several detailed examples are available [here](/docs/types/timestamp.md).
