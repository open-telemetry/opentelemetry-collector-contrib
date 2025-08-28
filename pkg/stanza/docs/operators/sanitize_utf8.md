## `sanitize_utf8` Operator

The `sanitize_utf8` operator is used to replace invalid UTF-8 characters in a specified field of a log entry. This is useful
for ensuring that log data is properly encoded and can be processed by downstream systems.

Invalid UTF-8 byte sequences will be replaced with `\uFFFD` (`�`).

### Configuration Fields

| Field      | Default          | Description                                                                                                                                                                                                                           |
|------------|------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `id`       | `sanitize_utf8`  | A unique identifier for the operator.                                                                                                                                                                                                 |
| `output`   | Next in pipeline | The connected operator(s) that will receive all outbound entries.                                                                                                                                                                     |
| `field`    | `body`           | The [field](../types/field.md) to sanitize. This must be a string field.                                                                                                                                                              |
| `on_error` | `send`           | The behavior of the operator if it encounters an error. See [on_error](../types/on_error.md).                                                                                                                                         |
| `if`       |                  | An [expression](../types/expression.md) that, when set, will be evaluated to determine whether this operator should be used for the given entry. This allows you to do easy conditional parsing without branching logic with routers. |

### Example Configurations

#### Simple Configuration for String Body

```yaml
- type: sanitize_utf8
  field: body
```