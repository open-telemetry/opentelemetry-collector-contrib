## `sanitize` Operator

The `sanitize` operator is used to replace invalid UTF-8 characters in a specified field of a log entry. This is useful for ensuring that log data is properly encoded and can be processed by downstream systems.

### Configuration Fields

| Field   | Default  | Description |
| ---     | ---      | ---         |
| `field`  | `body`   | The field to sanitize. This must be a string field. |

### Example Configurations

#### Simple Configuration for String Body

```yaml
- type: sanitize
  field: body
```