## `timestamp` parsing parameters

Parser operators, such as [`regex_parser`](../operators/regex_parser.md), can parse a timestamp and attach the resulting time value to the log entry in the `timestamp` field.

To configure timestamp parsing, add a `timestamp` block in the parser's configuration.

If a timestamp block is specified, the parser operator will perform the timestamp parsing _after_ performing its other parsing actions, but _before_ passing the entry to the specified output operator.

| Field         | Default    | Description |
| ---           | ---        | ---         |
| `parse_from`  | required   | The [field](../types/field.md) from which the value will be parsed. |
| `layout_type` | `strptime` | The type of timestamp. Valid values are `strptime`, `gotime`, and `epoch`. |
| `layout`      | required   | The exact layout of the timestamp to be parsed. |
| `location`    | `Local`    | The geographic location (timezone) to use when parsing a timestamp that does not include a timezone. The available locations depend on the local IANA Time Zone database. [This page](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones) contains many examples, such as `America/New_York`. |

### How to specify timestamp parsing parameters

```yaml
- type: regex_parser
  regexp: '^Time=(?P<timestamp_field>\d{4}-\d{2}-\d{2}), Host=(?P<host>[^,]+)'
  timestamp:
    parse_from: attributes.timestamp_field
    layout_type: strptime
    layout: '%Y-%m-%d'
```

---

As a special case, the [`time_parser`](../operators/time_parser.md) operator supports these fields inline. This is because time parsing is the primary purpose of the operator.

```yaml
- type: time_parser
  parse_from: body.timestamp_field
  layout_type: strptime
  layout: '%Y-%m-%d'
```

### Example Configurations

The following examples use `filelog` receiver, but they also apply to other components that use the stanza libarary.

#### Parse timestamps from plain text logs

Let's assume you have a `my-app.log` file with timestamps at the start of each line:

```logs
2022-01-02 07:24:56,123 -0700 [INFO] App started
2022-01-02 07:24:57,456 -0700 [INFO] Something happened
2022-01-02 07:24:58,789 -0700 [WARN] Look alive!
```

Since the timestamp is a part of the log's body, it needs to be extracted from the body with a [regex_parser](../operators/regex_parser.md) operator.

```yaml
exporters:
  logging:
    loglevel: debug
receivers:
  filelog:
    include:
    - my-app.log
    start_at: beginning
    operators:
    - type: regex_parser
      regex: (?P<timestamp_field>^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2},\d{3} (\+|\-)\d{2}\d{2})
      timestamp:
        layout: "%Y-%m-%d %H:%M:%S.%f %z"
        parse_from: attributes.timestamp_field
service:
  pipelines:
    logs:
      receivers:
      - filelog
      exporters:
      - logging
```

Note that this configuration has a side effect of creating a `timestamp_field` attribute for each log record.
To get rid of the attribute, use the `remove` operator:

```yaml
exporters:
  logging:
    loglevel: debug
receivers:
  filelog:
    include:
    - my-app.log
    start_at: beginning
    operators:
    - type: regex_parser
      regex: (?P<timestamp_field>^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2},\d{3} (\+|\-)\d{2}\d{2})
      timestamp:
        layout: "%Y-%m-%d %H:%M:%S.%f %z"
        parse_from: attributes.timestamp_field
    - type: remove
      field: attributes.timestamp_field
service:
  pipelines:
    logs:
      receivers:
      - filelog
      exporters:
      - logging
```

#### Parse timestamps from JSON logs

Let's assume you have logs in JSON format:

```logs
{"log":"[INFO] App started\n","stream":"stdout","time":"2022-01-02T14:24:56.123123123Z"}
{"log":"[INFO] Something happened\n","stream":"stdout","time":"2022-01-02T14:24:57.456456456Z"}
{"log":"[WARN] Look alive!\n","stream":"stdout","time":"2022-01-02T14:24:58.789789789Z"}
```

Use [json_parser](../operators/json_parser.md) to parse the log body into JSON and then parse the `time` field into the timestamp:

```yaml
exporters:
  logging:
    loglevel: debug
receivers:
  filelog:
    include:
    - logs-json.log
    start_at: beginning
    operators:
    - type: json_parser
      parse_to: body
    - type: time_parser
      parse_from: body.time
      layout: '%Y-%m-%dT%H:%M:%S.%LZ'
service:
  pipelines:
    logs:
      receivers:
      - filelog
      exporters:
      - logging
```

The above example uses a standalone [time_parser](../operators/time_parser.md) operator to parse the timestamp,
but you could also use a `timestamp` block inside the `json_parser`.

#### Parse a timestamp using a `strptime` layout

The default `layout_type` is `strptime`, which uses "directives" such as `%Y` (4-digit year) and `%H` (2-digit hour). A full list of supported directives is found [here](../../../../internal/coreinternal/timeutils/internal/ctimefmt/ctimefmt.go#L68).

Configuration:
```yaml
- type: time_parser
  parse_from: body.timestamp_field
  layout_type: strptime
  layout: '%a %b %e %H:%M:%S %Z %Y'
```

<table>
<tr><td> Input entry </td> <td> Output entry </td></tr>
<tr>
<td>

```json
{
  "timestamp": "",
  "body": {
    "timestamp_field": "Jun 5 13:50:27 EST 2020"
  }
}
```

</td>
<td>

```json
{
  "timestamp": "2020-06-05T13:50:27-05:00",
  "body": {
    "timestamp_field": "Jun 5 13:50:27 EST 2020"
  }
}
```

</td>
</tr>
</table>

#### Parse a timestamp using a `gotime` layout

The `gotime` layout type uses Golang's native time parsing capabilities. Golang takes an [unconventional approach](https://www.pauladamsmith.com/blog/2011/05/go_time.html) to time parsing. Finer details are well-documented [here](https://golang.org/src/time/format.go?s=25102:25148#L9).

Configuration:
```yaml
- type: time_parser
  parse_from: body.timestamp_field
  layout_type: gotime
  layout: Jan 2 15:04:05 MST 2006
```

<table>
<tr><td> Input entry </td> <td> Output entry </td></tr>
<tr>
<td>

```json
{
  "timestamp": "",
  "body": {
    "timestamp_field": "Jun 5 13:50:27 EST 2020"
  }
}
```

</td>
<td>

```json
{
  "timestamp": "2020-06-05T13:50:27-05:00",
  "body": {
    "timestamp_field": "Jun 5 13:50:27 EST 2020"
  }
}
```

</td>
</tr>
</table>

#### Parse a timestamp using an `epoch` layout

The `epoch` layout type uses can consume epoch-based timestamps. The following layouts are supported:

| Layout | Meaning                                   | Example              | `parse_from` data type support                           |
| ---    | ---                                       | ---                  | ---                                                      |
| `s`    | Seconds since the epoch                   | 1136214245           | `string`, `int64`, `float64`                             |
| `ms`   | Milliseconds since the epoch              | 1136214245123        | `string`, `int64`, `float64`                             |
| `us`   | Microseconds since the epoch              | 1136214245123456     | `string`, `int64`, `float64`                             |
| `ns`   | Nanoseconds since the epoch               | 1136214245123456789  | `string`, `int64`, `float64`<sup>[2]</sup>               |
| `s.ms` | Seconds plus milliseconds since the epoch | 1136214245.123       | `string`, `int64`<sup>[1]</sup>, `float64`               |
| `s.us` | Seconds plus microseconds since the epoch | 1136214245.123456    | `string`, `int64`<sup>[1]</sup>, `float64`               |
| `s.ns` | Seconds plus nanoseconds since the epoch  | 1136214245.123456789 | `string`, `int64`<sup>[1]</sup>, `float64`<sup>[2]</sup> |

<sub>[1] Interpretted as seconds. Equivalent to using `s` layout.</sub><br/>
<sub>[2] Due to floating point precision limitations, loss of up to 100ns may be expected.</sub>



Configuration:
```yaml
- type: time_parser
  parse_from: body.timestamp_field
  layout_type: epoch
  layout: s
```

<table>
<tr><td> Input entry </td> <td> Output entry </td></tr>
<tr>
<td>

```json
{
  "timestamp": "",
  "body": {
    "timestamp_field": 1136214245
  }
}
```

</td>
<td>

```json
{
  "timestamp": "2006-01-02T15:04:05-07:00",
  "body": {
    "timestamp_field": 1136214245
  }
}
```

</td>
</tr>
</table>
