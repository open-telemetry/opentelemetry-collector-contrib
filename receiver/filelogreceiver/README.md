# Filelog Receiver

| Status                   |           |
| ------------------------ |-----------|
| Stability                | [beta]    |
| Supported pipeline types | logs      |
| Distributions            | [contrib] |

Tails and parses logs from files.

## Configuration

| Field                               | Default                              | Description                                                                                                                                                                                                                                                     |
|-------------------------------------|--------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `include`                           | required                             | A list of file glob patterns that match the file paths to be read                                                                                                                                                                                               |
| `exclude`                           | []                                   | A list of file glob patterns to exclude from reading                                                                                                                                                                                                            |
| `start_at`                          | `end`                                | At startup, where to start reading logs from the file. Options are `beginning` or `end`                                                                                                                                                                         |
| `multiline`                         |                                      | A `multiline` configuration block. See below for more details                                                                                                                                                                                                   |
| `force_flush_period`                | `500ms`                              | Time since last read of data from file, after which currently buffered log should be send to pipeline. Takes `time.Duration` (e.g. `10s`, `1m`, or `500ms`) as value. Zero means waiting for new data forever                                                   |
| `encoding`                          | `utf-8`                              | The encoding of the file being read. See the list of supported encodings below for available options                                                                                                                                                            |
| `preserve_leading_whitespaces`      | `false`                              | Whether to preserve leading whitespaces.                                                                                                                                                                                                                        |
| `preserve_trailing_whitespaces`     | `false`                              | Whether to preserve trailing whitespaces.                                                                                                                                                                                                                       |
| `include_file_name`                 | `true`                               | Whether to add the file name as the attribute `log.file.name`.                                                                                                                                                                                                  |
| `include_file_path`                 | `false`                              | Whether to add the file path as the attribute `log.file.path`.                                                                                                                                                                                                  |
| `include_file_name_resolved`        | `false`                              | Whether to add the file name after symlinks resolution as the attribute `log.file.name_resolved`.                                                                                                                                                               |
| `include_file_path_resolved`        | `false`                              | Whether to add the file path after symlinks resolution as the attribute `log.file.path_resolved`.                                                                                                                                                               |
| `poll_interval`                     | 200ms                                | The duration between filesystem polls                                                                                                                                                                                                                           |
| `fingerprint_size`                  | `1kb`                                | The number of bytes with which to identify a file. The first bytes in the file are used as the fingerprint. Decreasing this value at any point will cause existing fingerprints to forgotten, meaning that all files will be read from the beginning (one time) |
| `max_log_size`                      | `1MiB`                               | The maximum size of a log entry to read. A log entry will be truncated if it is larger than `max_log_size`. Protects against reading large amounts of data into memory                                                                                          |
| `max_concurrent_files`              | 1024                                 | The maximum number of log files from which logs will be read concurrently. If the number of files matched in the `include` pattern exceeds this number, then files will be processed in batches.                                                                |
| `max_batches`                       | 0                                    | Only applicable when files must be batched in order to respect `max_concurrent_files`. This value limits the number of batches that will be processed during a single poll interval. A value of 0 indicates no limit.                                           |
| `delete_after_read`                 | `false`                              | If `true`, each log file will be read and then immediately deleted. Requires that the `filelog.allowFileDeletion` feature gate is enabled.                                                                                                                      |
| `attributes`                        | {}                                   | A map of `key: value` pairs to add to the entry's attributes                                                                                                                                                                                                    |
| `resource`                          | {}                                   | A map of `key: value` pairs to add to the entry's resource                                                                                                                                                                                                      |
| `operators`                         | []                                   | An array of [operators](../../pkg/stanza/docs/operators/README.md#what-operators-are-available). See below for more details                                                                                                                                     |
| `storage`                           | none                                 | The ID of a storage extension to be used to store file checkpoints. File checkpoints allow the receiver to pick up where it left off in the case of a collector restart. If no storage extension is used, the receiver will manage checkpoints in memory only.  |
| `header`                            | nil                                  | Specifies options for parsing header metadata. Requires that the `filelog.allowHeaderMetadataParsing` feature gate is enabled. See below for details.                                                                                                           |
| `header.pattern`                    | required for header metadata parsing | A regex that matches every header line.                                                                                                                                                                                                                         |
| `header.metadata_operators`         | required for header metadata parsing | A list of operators used to parse metadata from the header.                                                                                                                                                                                                     |
| `retry_on_failure.enabled`          | `false`                              | If `true`, the receiver will pause reading a file and attempt to resend the current batch of logs if it encounters an error from downstream components.                                                                                                         |
| `retry_on_failure.initial_interval` | `1 second`                           | Time to wait after the first failure before retrying.                                                                                                                                                                                                           |
| `retry_on_failure.max_interval`     | `30 seconds`                         | Upper bound on retry backoff interval. Once this value is reached the delay between consecutive retries will remain constant at the specified value.                                                                                                            |
| `retry_on_failure.max_elapsed_time` | `5 minutes`                          | Maximum amount of time (including retries) spent trying to send a logs batch to a downstream consumer. Once this value is reached, the data is discarded. Retrying never stops if set to `0`.                                                                   |

Note that _by default_, no logs will be read from a file that is not actively being written to because `start_at` defaults to `end`.

### Operators

Each operator performs a simple responsibility, such as parsing a timestamp or JSON. Chain together operators to process logs into a desired format.

- Every operator has a `type`.
- Every operator can be given a unique `id`. If you use the same type of operator more than once in a pipeline, you must specify an `id`. Otherwise, the `id` defaults to the value of `type`.
- Operators will output to the next operator in the pipeline. The last operator in the pipeline will emit from the receiver. Optionally, the `output` parameter can be used to specify the `id` of another operator to which logs will be passed directly.
- Only parsers and general purpose operators should be used.

### Multiline configuration

If set, the `multiline` configuration block instructs the `file_input` operator to split log entries on a pattern other than newlines.

The `multiline` configuration block must contain exactly one of `line_start_pattern` or `line_end_pattern`. These are regex patterns that
match either the beginning of a new log entry, or the end of a log entry.

### Supported encodings

| Key        | Description
| ---        | ---                                                              |
| `nop`      | No encoding validation. Treats the file as a stream of raw bytes |
| `utf-8`    | UTF-8 encoding                                                   |
| `utf-16le` | UTF-16 encoding with little-endian byte order                    |
| `utf-16be` | UTF-16 encoding with big-endian byte order                       |
| `ascii`    | ASCII encoding                                                   |
| `big5`     | The Big5 Chinese character encoding                              |

Other less common encodings are supported on a best-effort basis. See [https://www.iana.org/assignments/character-sets/character-sets.xhtml](https://www.iana.org/assignments/character-sets/character-sets.xhtml) for other encodings available.

### Header Metadata Parsing

To enable header metadata parsing, the `filelog.allowHeaderMetadataParsing` feature gate must be set, and `start_at` must be `beginning`.

If set, the file input operator will attempt to read a header from the start of the file. Each header line must match the `header.pattern` pattern. Each line is emitted into a pipeline defined by `header.metadata_operators`. Any attributes on the resultant entry from the embedded pipeline will be merged with the attributes from previous lines (attribute collisions will be resolved with an upsert strategy). After all header lines are read, the final merged header attributes will be present on every log line that is emitted for the file.

The header lines are not emitted by the receiver.

## Additional Terminology and Features

- An [entry](../../pkg/stanza/docs/types/entry.md) is the base representation of log data as it moves through a pipeline. All operators either create, modify, or consume entries.
- A [field](../../pkg/stanza/docs/types/field.md) is used to reference values in an entry.
- A common [expression](../../pkg/stanza/docs/types/expression.md) syntax is used in several operators. For example, expressions can be used to [filter](../../pkg/stanza/docs/operators/filter.md) or [route](../../pkg/stanza/docs/operators/router.md) entries.

### Parsers with Embedded Operations

Many parsers operators can be configured to embed certain followup operations such as timestamp and severity parsing. For more information, see [complex parsers](../../pkg/stanza/docs/types/parsers.md#complex-parsers).

## Example - Tailing a simple json file

Receiver Configuration
```yaml
receivers:
  filelog:
    include: [ /var/log/myservice/*.json ]
    operators:
      - type: json_parser
        timestamp:
          parse_from: attributes.time
          layout: '%Y-%m-%d %H:%M:%S'
```

[beta]: https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
