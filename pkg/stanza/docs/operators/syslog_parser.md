## `syslog_parser` operator

The `syslog_parser` operator parses the string-type field selected by `parse_from` as syslog. Timestamp parsing is handled automatically by this operator.

### Configuration Fields

| Field                                | Default          | Description |
| ---                                  | ---              | ---         |
| `id`                                 | `syslog_parser`  | A unique identifier for the operator. |
| `output`                             | Next in pipeline | The connected operator(s) that will receive all outbound entries. |
| `parse_from`                         | `body`           | The [field](../types/field.md) from which the value will be parsed. |
| `parse_to`                           | `attributes`     | The [field](../types/field.md) to which the value will be parsed. |
| `on_error`                           | `send`           | The behavior of the operator if it encounters an error. See [on_error](../types/on_error.md). |
| `protocol`                           | required         | The protocol to parse the syslog messages as. Options are `rfc3164` and `rfc5424`. |
| `location`                           | `UTC`            | The geographic location (timezone) to use when parsing the timestamp (Syslog RFC 3164 only). The available locations depend on the local IANA Time Zone database. [This page](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones) contains many examples, such as `America/New_York`. |
| `enable_octet_counting`              | `false`          | Wether or not to enable [RFC 6587](https://www.rfc-editor.org/rfc/rfc6587#section-3.4.1) Octet Counting on syslog parsing (Syslog RFC 5424 only).  |
| `allow_skip_pri_header`              | `false`          | Allow parsing records without the PRI header. If this setting is enabled, messages without the PRI header will be successfully parsed. The `severity` and `severity_text` fields as well as the `priority`, `facility`, and `facility_text` attributes will not be set. If this setting is disabled (the default), messages without PRI header will throw an exception. To set this setting to `true`, the `enable_octet_counting` setting must be `false`.|
| `non_transparent_framing_trailer`    | `nil`            | The framing trailer, either `LF` or `NUL`, when using [RFC 6587](https://www.rfc-editor.org/rfc/rfc6587#section-3.4.2) Non-Transparent-Framing (Syslog RFC 5424 only). |
| `timestamp`                          | `nil`            | An optional [timestamp](../types/timestamp.md) block which will parse a timestamp field before passing the entry to the output operator                                                                                               |
| `severity`                           | `nil`            | An optional [severity](../types/severity.md) block which will parse a severity field before passing the entry to the output operator                                                                                                  |
| `if`                                 |                  | An [expression](../types/expression.md) that, when set, will be evaluated to determine whether this operator should be used for the given entry. This allows you to do easy conditional parsing without branching logic with routers. |

### Embedded Operations

The `syslog_parser` can be configured to embed certain operations such as timestamp and severity parsing. For more information, see [complex parsers](../types/parsers.md#complex-parsers).

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
    "facility_text": "auth",
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
