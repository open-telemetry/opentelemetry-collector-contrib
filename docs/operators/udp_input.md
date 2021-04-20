## `udp_input` operator

The `udp_input` operator listens for logs from UDP packets.

### Configuration Fields

| Field             | Default          | Description                                                                                        |
| ---               | ---              | ---                                                                                                |
| `id`              | `udp_input`      | A unique identifier for the operator                                                               |
| `output`          | Next in pipeline | The connected operator(s) that will receive all outbound entries                                   |
| `listen_address`  | required         | A listen address of the form `<ip>:<port>`                                                         |
| `write_to`        | $                | The body [field](/docs/types/field.md) written to when creating a new log entry                    |
| `attributes`      | {}               | A map of `key: value` pairs to add to the entry's attributes                                       |
| `resource`        | {}               | A map of `key: value` pairs to add to the entry's resource                                         |
| `add_attributes`  | false            | Adds `net.transport`, `net.peer.ip`, `net.peer.port`, `net.host.ip` and `net.host.port` attributes |

### Example Configurations

#### Simple

Configuration:
```yaml
- type: udp_input
  listen_adress: "0.0.0.0:54526"
```

Send a log:
```bash
$ nc -u localhost 54525 <<EOF
heredoc> message1
heredoc> message2
heredoc> EOF
```

Generated entries:
```json
{
  "timestamp": "2020-04-30T12:10:17.656726-04:00",
  "body": "message1\nmessage2\n"
}
```
