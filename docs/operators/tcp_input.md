## `tcp_input` operator

The `tcp_input` operator listens for logs on one or more TCP connections. The operator assumes that logs are newline separated.

### Configuration Fields

| Field             | Default          | Description                                                                                        |
| ---               | ---              | ---                                                                                                |
| `id`              | `tcp_input`      | A unique identifier for the operator                                                               |
| `output`          | Next in pipeline | The connected operator(s) that will receive all outbound entries                                   |
| `max_buffer_size` | `1024kib`        | Maximum size of buffer that may be allocated while reading TCP input                               |
| `listen_address`  | required         | A listen address of the form `<ip>:<port>`                                                         |
| `tls`             | nil              | An optional `TLS` configuration (see the TLS configuration section)                                |
| `write_to`        | $                | The body [field](/docs/types/field.md) written to when creating a new log entry                    |
| `attributes`      | {}               | A map of `key: value` pairs to add to the entry's attributes                                       |
| `resource`        | {}               | A map of `key: value` pairs to add to the entry's resource                                         |
| `add_attributes`  | false            | Adds `net.transport`, `net.peer.ip`, `net.peer.port`, `net.host.ip` and `net.host.port` attributes |


#### TLS Configuration

The `tcp_input` operator supports TLS, disabled by default.
config more detail [opentelemetry-collector#configtls](https://github.com/open-telemetry/opentelemetry-collector/tree/main/config/configtls#tls-configuration-settings).

| Field             | Default          | Description                               |
| ---               | ---              | ---                                       |
| `cert_file`       |                  | Path to the TLS cert to use for TLS required connections.   |
| `key_file`        |                  | Path to the TLS key to use for TLS required connections.       |
| `ca_file`         |                  | Path to the CA cert. For a client this verifies the server certificate. For a server this verifies client certificates. If empty uses system root CA.        |
| `client_ca_file`  |                  | Path to the TLS cert to use by the server to verify a client certificate. (optional)   |


### Example Configurations

#### Simple

Configuration:
```yaml
- type: tcp_input
  listen_address: "0.0.0.0:54525"
```

Send a log:
```bash
$ nc localhost 54525 <<EOF
heredoc> message1
heredoc> message2
heredoc> EOF
```

Generated entries:
```json
{
  "timestamp": "2020-04-30T12:10:17.656726-04:00",
  "body": "message1"
},
{
  "timestamp": "2020-04-30T12:10:17.657143-04:00",
  "body": "message2"
}
```
