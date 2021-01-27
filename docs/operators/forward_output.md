## `forward_output` operator

The `forward_output` operator sends logs to another Stanza instance running `forward_input`.

### Configuration Fields

| Field     | Default          | Description                                                                              |
| ---       | ---              | ---                                                                                      |
| `id`      | `forward_output` | A unique identifier for the operator                                                     |
| `address`      | required | The address that the downstream Stanza instance is listening on |
| `buffer`  |                  | A [buffer](/docs/types/buffer.md) block indicating how to buffer entries before flushing |
| `flusher` |                  | A [flusher](/docs/types/flusher.md) block configuring flushing behavior                  |


### Example Configurations

#### Simple configuration

Configuration:
```yaml
- type: forward_output
  address: "http://downstream_server:25535"
```
