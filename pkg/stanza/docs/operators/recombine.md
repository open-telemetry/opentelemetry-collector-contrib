## `recombine` operator

The `recombine` operator combines consecutive logs into single logs based on simple expression rules.

### Configuration Fields

| Field                | Default          | Description |
| ---                  | ---              | ---         |
| `id`                 | `recombine`      | A unique identifier for the operator. |
| `output`             | Next in pipeline | The connected operator(s) that will receive all outbound entries. |
| `on_error`           | `send`           | The behavior of the operator if it encounters an error. See [on_error](../types/on_error.md). |
| `is_first_entry`     |                  | An [expression](../types/expression.md) that returns true if the entry being processed is the first entry in a multiline series. |
| `is_last_entry`      |                  | An [expression](../types/expression.md) that returns true if the entry being processed is the last entry in a multiline series. |
| `combine_field`      | required         | The [field](../types/field.md) from all the entries that will recombined. |
| `combine_with`       | `"\n"`           | The string that is put between the combined entries. This can be an empty string as well. When using special characters like `\n`, be sure to enclose the value in double quotes: `"\n"`. |
| `max_batch_size`     | 1000             | The maximum number of consecutive entries that will be combined into a single entry. |
| `overwrite_with`     | `oldest`         | Whether to use the fields from the `oldest` or the `newest` entry for all the fields that are not combined. |
| `force_flush_period` | `5s`             | Flush timeout after which entries will be flushed aborting the wait for their sub parts to be merged with. |
| `source_identifier`  | `$attributes["file.path"]` | The [field](../types/field.md) to separate one source of logs from others when combining them. |
| `max_sources`        | 1000             | The maximum number of unique sources allowed concurrently to be tracked for combining separately. |
| `max_log_size`       | 0                | The maximum bytes size of the combined field. Once the size exceeds the limit, all received entries of the source will be combined and flushed. "0" of max_log_size means no limit. |

Exactly one of `is_first_entry` and `is_last_entry` must be specified.

NOTE: this operator is only designed to work with a single input. It does not keep track of what operator entries are coming from, so it can't combine based on source.

### Example Configurations

#### Recombine Kubernetes logs in the CRI format

Kubernetes logs in the CRI format have a tag that indicates whether the log entry is part of a longer log line (P) or the final entry (F). Using this tag, we can recombine the CRI logs back into complete log lines.

Configuration:

```yaml
- type: file_input
  include:
    - ./input.log
- type: regex_parser
  regex: '^(?P<timestamp>[^\s]+) (?P<stream>\w+) (?P<logtag>\w) (?P<message>.*)'
- type: recombine
  combine_field: body.message
  combine_with: ""
  is_last_entry: "body.logtag == 'F'"
  overwrite_with: "newest"
```

Input file:

```
2016-10-06T00:17:09.669794202Z stdout F Single entry log 1
2016-10-06T00:17:10.113242941Z stdout P This is a very very long line th
2016-10-06T00:17:10.113242941Z stdout P at is really really long and spa
2016-10-06T00:17:10.113242941Z stdout F ns across multiple log entries
```

Output logs:

```json
[
  {
    "timestamp": "2020-12-04T13:03:38.41149-05:00",
    "severity": 0,
    "body": {
      "message": "Single entry log 1",
      "logtag": "F",
      "stream": "stdout",
      "timestamp": "2016-10-06T00:17:09.669794202Z"
    }
  },
  {
    "timestamp": "2020-12-04T13:03:38.411664-05:00",
    "severity": 0,
    "body": {
      "message": "This is a very very long line that is really really long and spans across multiple log entries",
      "logtag": "F",
      "stream": "stdout",
      "timestamp": "2016-10-06T00:17:10.113242941Z"
    }
  }
]
```

#### Recombine stack traces into multiline logs

Some apps output multiple log lines which are in fact a single log record. A common example is a stack trace:

```console
java.lang.Exception: Stack trace
        at java.lang.Thread.dumpStack(Thread.java:1336)
        at Main.demo3(Main.java:15)
        at Main.demo2(Main.java:12)
        at Main.demo1(Main.java:9)
        at Main.demo(Main.java:6)
        at Main.main(Main.java:3)
```

To recombine such log lines into a single log record, you need a way to tell when a log record starts or ends.
In the example above, the first line differs from the other lines in not starting with a whitespace.
This can be expressed with the following configuration:

```yaml
- type: recombine
  combine_field: body.message
  is_first_entry: body.message matches "^[^\s]"
```

Given the following input file:

```
Log message 1
Error: java.lang.Exception: Stack trace
        at java.lang.Thread.dumpStack(Thread.java:1336)
        at Main.demo3(Main.java:15)
        at Main.demo2(Main.java:12)
        at Main.demo1(Main.java:9)
        at Main.demo(Main.java:6)
        at Main.main(Main.java:3)
Another log message
```

The following logs will be output:

```json
[
  {
    "timestamp": "2020-12-04T13:03:38.41149-05:00",
    "severity": 0,
    "body": {
      "message": "Log message 1",
    }
  },
  {
    "timestamp": "2020-12-04T13:03:38.41149-05:00",
    "severity": 0,
    "body": {
      "message": "Error: java.lang.Exception: Stack trace\n        at java.lang.Thread.dumpStack(Thread.java:1336)\n        at Main.demo3(Main.java:15)\n        at Main.demo2(Main.java:12)\n        at Main.demo1(Main.java:9)\n        at Main.demo(Main.java:6)\n        at Main.main(Main.java:3)",
    }
  },
  {
    "timestamp": "2020-12-04T13:03:38.41149-05:00",
    "severity": 0,
    "body": {
      "message": "Another log message",
    }
  },
]
```
