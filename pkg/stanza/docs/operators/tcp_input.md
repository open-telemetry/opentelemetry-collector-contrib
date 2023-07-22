## `tcp_input` operator

The `tcp_input` operator listens for logs on one or more TCP connections. The operator assumes that logs are newline separated.

### Configuration Fields

| Field                                   | Default              | Description |
| ---                                     | ---                  | ---         |
| `id`                                    | `tcp_input`          | A unique identifier for the operator. |
| `output`                                | Next in pipeline     | The connected operator(s) that will receive all outbound entries. |
| `max_log_size`                          | `1MiB`               | The maximum size of a log entry to read before failing. Protects against reading large amounts of data into memory. |
| `listen_address`                        | required             | A listen address of the form `<ip>:<port>`. |
| `tls`                                   | nil                  | An optional `TLS` configuration (see the TLS configuration section). |
| `attributes`                            | {}                   | A map of `key: value` pairs to add to the entry's attributes. |
| `one_log_per_packet`                    | false               | Skip log tokenization, set to true if logs contains one log per record and multiline is not used.  This will improve performance. |
| `resource`                              | {}                   | A map of `key: value` pairs to add to the entry's resource. |
| `add_attributes`                        | false                | Adds `net.*` attributes according to [semantic convention][https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/span-general.md#general-network-connection-attributes]. |
| `multiline`                     |                  | A `multiline` configuration block. See below for details. |
| `preserve_leading_whitespaces`          | false                | Whether to preserve leading whitespaces.                                                                                                                                                                                                                         |
| `preserve_trailing_whitespaces`         | false                | Whether to preserve trailing whitespaces.                                                                                                                                                                                                                            |
| `encoding`                              | `utf-8`              | The encoding of the file being read. See the list of supported encodings below for available options. |

#### TLS Configuration

The `tcp_input` operator supports TLS, disabled by default.
config more detail [opentelemetry-collector#configtls](https://github.com/open-telemetry/opentelemetry-collector/tree/main/config/configtls#tls-configuration-settings).

| Field             | Default          | Description                                                                                                                                           |
| ---               | ---              | ---                                                                                                                                                   |
| `cert_file`       |                  | Path to the TLS cert to use for TLS required connections.                                                                                             |
| `key_file`        |                  | Path to the TLS key to use for TLS required connections.                                                                                              |
| `ca_file`         |                  | Path to the CA cert. For a client this verifies the server certificate. For a server this verifies client certificates. If empty uses system root CA. |
| `client_ca_file`  |                  | Path to the TLS cert to use by the server to verify a client certificate. (optional)                                                                  |

#### `multiline` configuration

If set, the `multiline` configuration block instructs the `tcp_input` operator to split log entries on a pattern other than newlines.

The `multiline` configuration block must contain exactly one of `line_start_pattern` or `line_end_pattern`. These are regex patterns that
match either the beginning of a new log entry, or the end of a log entry.

#### Supported encodings

| Key        | Description
| ---        | ---                                                              |
| `nop`      | No encoding validation. Treats the file as a stream of raw bytes |
| `utf-8`    | UTF-8 encoding                                                   |
| `utf-16le` | UTF-16 encoding with little-endian byte order                    |
| `utf-16be` | UTF-16 encoding with little-endian byte order                    |
| `ascii`    | ASCII encoding                                                   |
| `big5`     | The Big5 Chinese character encoding                              |

Other less common encodings are supported on a best-effort basis.
See [https://www.iana.org/assignments/character-sets/character-sets.xhtml](https://www.iana.org/assignments/character-sets/character-sets.xhtml)
for other encodings available.

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
