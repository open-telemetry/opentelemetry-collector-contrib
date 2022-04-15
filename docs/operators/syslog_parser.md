## `syslog_parser` operator

The `syslog_parser` operator parses the string-type field selected by `parse_from` as syslog. Timestamp parsing is handled automatically by this operator.

### Configuration Fields

| Field         | Default          | Description |
| ---           | ---              | ---         |
| `id`          | `syslog_parser`  | A unique identifier for the operator. |
| `output`      | Next in pipeline | The connected operator(s) that will receive all outbound entries. |
| `parse_from`  | `body`           | The [field](/docs/types/field.md) from which the value will be parsed. |
| `parse_to`    | `body`           | The [field](/docs/types/field.md) to which the value will be parsed. |
| `on_error`    | `send`           | The behavior of the operator if it encounters an error. See [on_error](/docs/types/on_error.md). |
| `protocol`    | required         | The protocol to parse the syslog messages as. Options are `rfc3164` and `rfc5424`. |
| `location`    | `UTC`            | The geographic location (timezone) to use when parsing the timestamp (Syslog RFC 3164 only). The available locations depend on the local IANA Time Zone database. [This page](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones) contains many examples, such as `America/New_York`. |
| `timestamp`   | `nil`            | An optional [timestamp](/docs/types/timestamp.md) block which will parse a timestamp field before passing the entry to the output operator                                                                                               |
| `severity`    | `nil`            | An optional [severity](/docs/types/severity.md) block which will parse a severity field before passing the entry to the output operator                                                                                                  |
| `if`          |                  | An [expression](/docs/types/expression.md) that, when set, will be evaluated to determine whether this operator should be used for the given entry. This allows you to do easy conditional parsing without branching logic with routers. |

### Example Configurations


#### Parse the field `message` as syslog

Configuration:
```yaml
- type: syslog_parser
  protocol: rfc3164
```

<table>
<tr><td> Input body </td> <td> Output body </td></tr>
<tr>
<td>

```json
{
  "timestamp": "",
  "body": "<34>Jan 12 06:30:00 1.2.3.4 apache_server: test message"
}
```

</td>
<td>

```json
{
  "timestamp": "2020-01-12T06:30:00Z",
  "body": {
    "appname": "apache_server",
    "facility": 4,
    "hostname": "1.2.3.4",
    "message": "test message",
    "msg_id": null,
    "priority": 34,
    "proc_id": null,
    "severity": 2
  }
}
```

</td>
</tr>
</table>
