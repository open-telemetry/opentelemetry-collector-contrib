## `udp_input` operator

The `udp_input` operator listens for logs from UDP packets.

### Configuration Fields

| Field                                   | Default              | Description |
| ---                                     | ---                  | ---         |
| `id`                                    | `udp_input`          | A unique identifier for the operator. |
| `output`                                | Next in pipeline     | The connected operator(s) that will receive all outbound entries. |
| `listen_address`                        | required             | A listen address of the form `<ip>:<port>`. |
| `attributes`                            | {}                   | A map of `key: value` pairs to add to the entry's attributes. |
| `one_log_per_packet`                    | false                | Skip log tokenization, set to true if logs contains one log per record and multiline is not used.  This will improve performance. |
| `resource`                              | {}                   | A map of `key: value` pairs to add to the entry's resource. |
| `add_attributes`                        | false                | Adds `net.*` attributes according to [semantic convention][https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/span-general.md#general-network-connection-attributes]. |
| `multiline`                     |                  | A `multiline` configuration block. See below for details. |
| `preserve_leading_whitespaces`          | false            | Whether to preserve leading whitespaces.                                                                                                                                                                                                                         |
| `preserve_trailing_whitespaces`             | false            | Whether to preserve trailing whitespaces.                                                                                                                                                                                                                            |
| `encoding`                              | `utf-8`              | The encoding of the file being read. See the list of supported encodings below for available options. |

#### `multiline` configuration

If set, the `multiline` configuration block instructs the `udp_input` operator to split log entries on a pattern other than newlines.

**note** If `multiline` is not set at all, it wont't split log entries at all. Every UDP packet is going to be treated as log.
**note** `multiline` detection works per UDP packet due to protocol limitations.

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
