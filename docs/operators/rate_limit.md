## `rate_limit` operator

The `rate_limit` operator limits the rate of entries that can pass through it. This is useful if you want to limit
throughput of the agent, or in conjunction with operators like `generate_input`, which will otherwise
send as fast as possible.

### Configuration Fields

| Field      | Default          | Description                                                                        |
| ---        | ---              | ---                                                                                |
| `id`       | `rate_limit`     | A unique identifier for the operator                                               |
| `output`   | Next in pipeline | The connected operator(s) that will receive all outbound entries                   |
| `rate`     |                  | The number of logs to allow per second                                             |
| `interval` |                  | A [duration](/docs/types/duration.md) that indicates the time between sent entries |
| `burst`    | 0                | The max number of entries to "save up" for spikes of load                          |

Exactly one of `rate` or `interval` must be specified.

### Example Configurations


#### Limit throughput to 10 entries per second

Configuration:
```yaml
- type: rate_limit
  rate: 10
```
