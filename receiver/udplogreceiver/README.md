# UDP Receiver

Receives logs from udp using
the [opentelemetry-log-collection](https://github.com/open-telemetry/opentelemetry-log-collection) library.

Supported pipeline types: logs

> :construction: This receiver is in alpha and configuration fields are subject to change.

## Configuration

| Field             | Default          | Description                                                                                        |
| ---               | ---              | ---                                                                                                |
| `output`          | Next in pipeline | The connected operator(s) that will receive all outbound entries                                   |
| `listen_address`  | required         | A listen address of the form `<ip>:<port>`                                                         |
| `write_to`        | $                | The body [field](/docs/types/field.md) written to when creating a new log entry                    |
| `attributes`      | {}               | A map of `key: value` pairs to add to the entry's attributes                                       |
| `resource`        | {}               | A map of `key: value` pairs to add to the entry's resource                                         |

## Example Configurations

### Simple

Configuration:

```yaml
receivers:
  udplog:
    listen_address: "0.0.0.0:54525"
```
