## `recombine` operator

The `recombine` operator combines consecutive logs into single logs based on simple expression rules.

### Configuration Fields

| Field            | Default          | Description |
| ---              | ---              | ---         |
| `id`             | `metadata`       | A unique identifier for the operator. |
| `output`         | Next in pipeline | The connected operator(s) that will receive all outbound entries. |
| `on_error`       | `send`           | The behavior of the operator if it encounters an error. See [on_error](/docs/types/on_error.md). |
| `is_first_entry` |                  | An [expression](/docs/types/expression.md) that returns true if the entry being processed is the first entry in a multiline series. |
| `is_last_entry`  |                  | An [expression](/docs/types/expression.md) that returns true if the entry being processed is the last entry in a multiline series. |
| `combine_field`  | required         | The [field](/docs/types/field.md) from all the entries that will recombined with newlines. |
| `max_batch_size` | 1000             | The maximum number of consecutive entries that will be combined into a single entry. |
| `overwrite_with` | `oldest`         | Whether to use the fields from the `oldest` or the `newest` entry for all the fields that are not combined with newlines. |

Exactly one of `is_first_entry` and `is_last_entry` must be specified.

NOTE: this operator is only designed to work with a single input. It does not keep track of what operator entries are coming from, so it can't combine based on source.

### Example Configurations


#### Recombine logs in the CRI format

Logs in the CRI format have a column that indicates whether the log is a partial log (P) or the last log in a series of partial logs (F). Using this column, we can recombine the CRI logs back into complete log messages.

Configuration:
```yaml
- type: file_input
  include: 
    - ./input.log
- type: regex_parser 
  regex: '^(?P<timestamp>[^\s]+) (?P<stream>\w+) (?P<partial>\w) (?P<message>.*)'
- type: recombine
  combine_field: message
  is_last_entry: "$body.partial == 'F'"
```

Input file: 
```
2016-10-06T00:17:09.669794202Z stdout F The content of the log entry 1
2016-10-06T00:17:10.113242941Z stdout P First line of log entry 2
2016-10-06T00:17:10.113242941Z stdout P Second line of the log entry 2
2016-10-06T00:17:10.113242941Z stdout F Last line of the log entry 2
```

Output logs: 
```json
[
  {
    "timestamp": "2020-12-04T13:03:38.41149-05:00",
    "severity": 0,
    "body": {
      "message": "The content of the log entry 1",
      "partial": "F",
      "stream": "stdout",
      "timestamp": "2016-10-06T00:17:09.669794202Z"
    }
  },
  {
    "timestamp": "2020-12-04T13:03:38.411664-05:00",
    "severity": 0,
    "body": {
      "message": "First line of log entry 2\nSecond line of the log entry 2\nLast line of the log entry 2",
      "partial": "P",
      "stream": "stdout",
      "timestamp": "2016-10-06T00:17:10.113242941Z"
    }
  }
]
```
