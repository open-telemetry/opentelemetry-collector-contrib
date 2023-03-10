## `file_input` operator

The `file_input` operator reads logs from files. It will place the lines read into the `body` of the new entry.

### Configuration Fields

| Field                           | Default          | Description |
| ---                             | ---              | ---         |
| `id`                            | `file_input`     | A unique identifier for the operator. |
| `output`                        | Next in pipeline | The connected operator(s) that will receive all outbound entries. |
| `include`                       | required         | A list of file glob patterns that match the file paths to be read. |
| `exclude`                       | []               | A list of file glob patterns to exclude from reading. |
| `poll_interval`                 | 200ms            | The duration between filesystem polls. |
| `multiline`                     |                  | A `multiline` configuration block. See below for details. |
| `force_flush_period`            | `500ms`          | Time since last read of data from file, after which currently buffered log should be send to pipeline. Takes `time.Time` as value. Zero means waiting for new data forever. |
| `encoding`                      | `utf-8`          | The encoding of the file being read. See the list of supported encodings below for available options. |
| `include_file_name`             | `true`           | Whether to add the file name as the attribute `log.file.name`. |
| `include_file_path`             | `false`          | Whether to add the file path as the attribute `log.file.path`. |
| `include_file_name_resolved`    | `false`          | Whether to add the file name after symlinks resolution as the attribute `log.file.name_resolved`. |
| `include_file_path_resolved`    | `false`          | Whether to add the file path after symlinks resolution as the attribute `log.file.path_resolved`. |
| `preserve_leading_whitespaces`  | `false`          | Whether to preserve leading whitespaces.                                                                                                                                                                                                                         |
| `preserve_trailing_whitespaces` | `false`          | Whether to preserve trailing whitespaces.                                                                                                                                                                                                                            |
| `start_at`                      | `end`            | At startup, where to start reading logs from the file. Options are `beginning` or `end`. This setting will be ignored if previously read file offsets are retrieved from a persistence mechanism. |
| `fingerprint_size`              | `1kb`            | The number of bytes with which to identify a file. The first bytes in the file are used as the fingerprint. Decreasing this value at any point will cause existing fingerprints to forgotten, meaning that all files will be read from the beginning (one time). |
| `max_log_size`                  | `1MiB`           | The maximum size of a log entry to read before failing. Protects against reading large amounts of data into memory |.
| `max_concurrent_files`          | 1024             | The maximum number of log files from which logs will be read concurrently (minimum = 2). If the number of files matched in the `include` pattern exceeds half of this number, then files will be processed in batches. |
| `max_batches`                   | 0                | Only applicable when files must be batched in order to respect `max_concurrent_files`. This value limits the number of batches that will be processed during a single poll interval. A value of 0 indicates no limit. |
| `delete_after_read`             | `false`          | If `true`, each log file will be read and then immediately deleted. Requires that the `filelog.allowFileDeletion` feature gate is enabled. |
| `attributes`                    | {}               | A map of `key: value` pairs to add to the entry's attributes. |
| `resource`                      | {}               | A map of `key: value` pairs to add to the entry's resource. |
| `header`                        | nil              | Specifies options for parsing header metadata. Requires that the `filelog.allowHeaderMetadataParsing` feature gate is enabled. See below for details. |
| `header.pattern`      | required for header metadata parsing | A regex that matches every header line. |
| `header.metadata_operators`     | required for header metadata parsing | A list of operators used to parse metadata from the header. |

Note that by default, no logs will be read unless the monitored file is actively being written to because `start_at` defaults to `end`.

`include` and `exclude` fields use `github.com/bmatcuk/doublestar` for expression language.
For reference documentation see [here](https://github.com/bmatcuk/doublestar#patterns).

#### `multiline` configuration

If set, the `multiline` configuration block instructs the `file_input` operator to split log entries on a pattern other than newlines.

The `multiline` configuration block must contain exactly one of `line_start_pattern` or `line_end_pattern`. These are regex patterns that
match either the beginning of a new log entry, or the end of a log entry.

If using multiline, last log can sometimes be not flushed due to waiting for more content.
In order to forcefully flush last buffered log after certain period of time,
use `force_flush_period` option.

Also refer to [recombine](../operators/recombine.md) operator for merging events with greater control.

### File rotation

When files are rotated and its new names are no longer captured in `include` pattern (i.e. tailing symlink files), it could result in data loss.
To avoid the data loss, choose move/create rotation method and set `max_concurrent_files` higher than the twice of the number of files to tail.

### Supported encodings

| Key        | Description
| ---        | ---                                                              |
| `nop`      | No encoding validation. Treats the file as a stream of raw bytes |
| `utf-8`    | UTF-8 encoding                                                   |
| `utf-16le` | UTF-16 encoding with little-endian byte order                    |
| `utf-16be` | UTF-16 encoding with little-endian byte order                    |
| `ascii`    | ASCII encoding                                                   |
| `big5`     | The Big5 Chinese character encoding                              |

Other less common encodings are supported on a best-effort basis. See [https://www.iana.org/assignments/character-sets/character-sets.xhtml](https://www.iana.org/assignments/character-sets/character-sets.xhtml) for other encodings available.

### Header Metadata Parsing

To enable header metadata parsing, the `filelog.allowHeaderMetadataParsing` feature gate must be set, and `start_at` must be `beginning`.

If set, the file input operator will attempt to read a header from the start of the file. Each header line must match the `header.pattern` pattern. Each line is emitted into a pipeline defined by `header.metadata_operators`. Any attributes on the resultant entry from the embedded pipeline will be merged with the attributes from previous lines (attribute collisions will be resolved with an upsert strategy). After all header lines are read, the final merged header attributes will be present on every log line that is emitted for the file.

The header lines are not emitted to the output operator.

### Example Configurations

#### Simple file input

Configuration:
```yaml
- type: file_input
  include:
    - ./test.log
```

<table>
<tr><td> `./test.log` </td> <td> Output bodies </td></tr>
<tr>
<td>

```
log1
log2
log3
```

</td>
<td>

```json
{
  "body": "log1"
},
{
  "body": "log2"
},
{
  "body": "log3"
}
```

</td>
</tr>
</table>

#### Multiline file input

Configuration:
```yaml
- type: file_input
  include:
    - ./test.log
  multiline:
    line_start_pattern: 'START '
```

<table>
<tr><td> `./test.log` </td> <td> Output bodies </td></tr>
<tr>
<td>

```
START log1
log2
START log3
log4
```

</td>
<td>

```json
{
  "body": "START log1\nlog2\n"
},
{
  "body": "START log3\nlog4\n"
}
```

</td>
</tr>
</table>
