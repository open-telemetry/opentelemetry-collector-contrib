# TCP Receiver

Receives logs from tcp using
the [opentelemetry-log-collection](https://github.com/open-telemetry/opentelemetry-log-collection) library.

Supported pipeline types: logs

> :construction: This receiver is in alpha and configuration fields are subject to change.

## Configuration

| Field             | Default          | Description                                                                                        |
| ---               | ---              | ---                                                                                                |
| `output`          | Next in pipeline | The connected operator(s) that will receive all outbound entries                                   |
| `max_buffer_size` | `1024kib`        | Maximum size of buffer that may be allocated while reading TCP input                               |
| `listen_address`  | required         | A listen address of the form `<ip>:<port>`                                                         |
| `tls`             | nil              | An optional `TLS` configuration (see the TLS configuration section)                                |
| `write_to`        | $                | The body [field](/docs/types/field.md) written to when creating a new log entry                    |
| `attributes`      | {}               | A map of `key: value` pairs to add to the entry's attributes                                       |
| `resource`        | {}               | A map of `key: value` pairs to add to the entry's resource                                         |

### TLS Configuration

The `tcplog` receiver supports TLS, disabled by default.
config more detail [opentelemetry-collector#configtls](https://github.com/open-telemetry/opentelemetry-collector/tree/main/config/configtls#tls-configuration-settings).

| Field             | Default          | Description                               |
| ---               | ---              | ---                                       |
| `cert_file`       |                  | Path to the TLS cert to use for TLS required connections.   |
| `key_file`        |                  | Path to the TLS key to use for TLS required connections.       |
| `ca_file`         |                  | Path to the CA cert. For a client this verifies the server certificate. For a server this verifies client certificates. If empty uses system root CA.        |
| `client_ca_file`  |                  | Path to the TLS cert to use by the server to verify a client certificate. (optional)   |

## Example Configurations

### Simple

Configuration:

```yaml
receivers:
  tcplog:
    listen_address: "0.0.0.0:54525"
```
