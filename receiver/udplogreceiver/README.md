# UDP Receiver

Receives logs from udp using
the [opentelemetry-log-collection](https://github.com/open-telemetry/opentelemetry-log-collection) library.

Supported pipeline types: logs

> :construction: This receiver is in alpha and configuration fields are subject to change.

## Configuration Fields

| Field             | Default          | Description                                                                                                        |
| ---               | ---              | ---                                                                                                                |
| `listen_address`  | required         | A listen address of the form `<ip>:<port>`                                                                         |
| `write_to`        | `$body`          | The body [field](/docs/types/field.md) written to when creating a new log entry                                    |
| `attributes`      | {}               | A map of `key: value` pairs to add to the entry's attributes                                                       |
| `resource`        | {}               | A map of `key: value` pairs to add to the entry's resource                                                         |
| `add_attributes`  | false            | Adds `net.*` attributes according to [semantic convention][https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/span-general.md#general-network-connection-attributes] |
| `multiline`       |                  | A `multiline` configuration block. See below for details                                                           |
| `encoding`        | `nop`            | The encoding of the file being read. See the list of supported encodings below for available options               |
| `operators`       | []               | An array of [operators](https://github.com/open-telemetry/opentelemetry-log-collection/blob/main/docs/operators/README.md#what-operators-are-available). See below for more details |

### Operators

Each operator performs a simple responsibility, such as parsing a timestamp or JSON. Chain together operators to process logs into a desired format.

- Every operator has a `type`.
- Every operator can be given a unique `id`. If you use the same type of operator more than once in a pipeline, you must specify an `id`. Otherwise, the `id` defaults to the value of `type`.
- Operators will output to the next operator in the pipeline. The last operator in the pipeline will emit from the receiver. Optionally, the `output` parameter can be used to specify the `id` of another operator to which logs will be passed directly.
- Only parsers and general purpose operators should be used.

### `multiline` configuration

If set, the `multiline` configuration block instructs the `udplog` receiver to split log entries on a pattern other than newlines.

**note** If `multiline` is not set at all, it wont't split log entries at all. Every UDP packet is going to be treated as log.
**note** `multiline` detection works per UDP packet due to protocol limitations.

The `multiline` configuration block must contain exactly one of `line_start_pattern` or `line_end_pattern`. These are regex patterns that
match either the beginning of a new log entry, or the end of a log entry.

### Supported encodings

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

## Example Configurations

### Simple

Configuration:

```yaml
receivers:
  udplog:
    listen_address: "0.0.0.0:54525"
```
