## `generate_input` operator

The `generate_input` operator generates log entries with a static body. This is useful for testing pipelines, especially when
coupled with the [`rate_limit`](/docs/operators/rate_limit.md) operator.

### Configuration Fields

| Field             | Default          | Description |
| ---               | ---              | ---         |
| `id`              | `generate_input` | A unique identifier for the operator. |
| `output`          | Next in pipeline | The connected operator(s) that will receive all outbound entries. |
| `entry`           |                  | A [entry](/docs/types/entry.md) log entry to repeatedly generate. |
| `count`           | 0                | The number of entries to generate before stopping. A value of 0 indicates unlimited. |
| `static`          | `false`          | If true, the timestamp of the entry will remain static after each invocation. |

### Example Configurations

#### Mock a file input

Configuration:
```yaml
- type: generate_input
  entry:
    body:
      message1: log1
      message2: log2
```

Output bodies:
```json
{
  "body": {
    "message1": "log1",
    "message2": "log2"
  },
},
{
  "body": {
    "message1": "log1",
    "message2": "log2"
  },
},
...
```
