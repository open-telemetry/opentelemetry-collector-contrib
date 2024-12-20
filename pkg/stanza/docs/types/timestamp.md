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

## Layout Types

### `strptime` and `gotime`

The `strptime` layout type approximates familiar strptime/strftime formats. See the table below for a list of supported directives.

The `gotime` layout type uses Golang's native time parsing capabilities. Golang takes an [unconventional approach](https://www.pauladamsmith.com/blog/2011/05/go_time.html) to time parsing. Finer details are documented [here](https://golang.org/src/time/format.go?s=25102:25148#L9).

| `strptime` directive | `gotime` equivalent | Description |
| --- | --- | --- |
| `%Y` | `2006` | Year, zero-padded (0001, 0002, ..., 2019, 2020, ..., 9999) |
| `%y` | `06` | Year, last two digits, zero-padded (01, ..., 99) |
| `%m` | `01` | Month as a decimal number (01, 02, ..., 12) |
| `%o` | `_1` | Month as a space-padded number ( 1, 2, ..., 12) |
| `%q` | `1` | Month as a unpadded number (1,2,...,12) |
| `%b` | `Jan` | Abbreviated month name (Jan, Feb, ...) |
| `%B` | `January` | Full month name (January, February, ...) |
| `%d` | `02` | Day of the month, zero-padded (01, 02, ..., 31) |
| `%e` | `_2` | Day of the month, space-padded ( 1, 2, ..., 31) |
| `%g` | `2` | Day of the month, unpadded (1,2,...,31) |
| `%a` | `Mon` | Abbreviated weekday name (Sun, Mon, ...) |
| `%A` | `Monday` | Full weekday name (Sunday, Monday, ...) |
| `%H` | `15` | Hour (24-hour clock) as a zero-padded decimal number (00, ..., 24) |
| `%I` | `3` | Hour (12-hour clock) as a zero-padded decimal number (00, ..., 12) |
| `%l` | `03` | Hour (12-hour clock: 0, ..., 12) |
| `%p` | `PM` | Locale’s equivalent of either AM or PM |
| `%P` | `pm` | Locale’s equivalent of either am or pm |
| `%M` | `04` | Minute, zero-padded (00, 01, ..., 59) |
| `%S` | `05` | Second as a zero-padded decimal number (00, 01, ..., 59) |
| `%L` | `999` | Millisecond as a decimal number, zero-padded on the left (000, 001, ..., 999) |
| `%f` | `999999` | Microsecond as a decimal number, zero-padded on the left (000000, ..., 999999) |
| `%s` | `99999999` | Nanosecond as a decimal number, zero-padded on the left (000000, ..., 999999) |
| `%Z` | `MST` | Timezone name or abbreviation or empty (UTC, EST, CST) |
| `%z` | `Z0700` | UTC offset in the form ±HHMM[SS[.ffffff]] or empty(+0000, -0400) |
| `%i` | `-07` | UTC offset in the form ±HH or empty(+00, -04) |
| `%j` | `-07:00` | UTC offset in the form ±HH:MM or empty(+00:00, -04:00) |
| `%k` | `-07:00:00` | UTC offset in the form ±HH:MM:SS or empty(+00:00:00, -04:00:00) |
| `%D` | `01/02/2006` | Short MM/DD/YY date, equivalent to `%m/%d/%y` |
| `%F` | `2006-01-02` | Short YYYY-MM-DD date, equivalent to `%Y-%m-%d` |
| `%T` | `15:04:05` | ISO 8601 time format (HH:MM:SS), equivalent to `%H:%M:%S` |
| `%r` | `03:04:05 pm` | 12-hour clock time, equivalent to `%l:%M:%S %p` |
| `%c` | `Mon Jan 02 15:04:05 2006` | Date and time representation, equivalent to `%a %b %d %H:%M:%S %Y` |
| `%R` | `15:04` | 24-hour HH:MM time, equivalent to `%H:%M` |
| `%n` | `\n` | New-line character |
| `%t` | `\t` | Horizontal-tab character |
| `%%` | `%` | Percent sign |

### `epoch`

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


### How to specify timestamp parsing parameters

```yaml
- type: regex_parser
  regex: '^Time=(?P<timestamp_field>\d{4}-\d{2}-\d{2}), Host=(?P<host>[^,]+)'
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

The following examples use file log receiver, but they also apply to other components that use the stanza libarary.

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
  debug:
    verbosity: detailed
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
      - debug
```

Note that this configuration has a side effect of creating a `timestamp_field` attribute for each log record.
To get rid of the attribute, use the `remove` operator:

```yaml
exporters:
  debug:
    verbosity: detailed
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
      - debug
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
  debug:
    verbosity: detailed
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
      - debug
```

The above example uses a standalone [time_parser](../operators/time_parser.md) operator to parse the timestamp,
but you could also use a `timestamp` block inside the `json_parser`.

#### Parse a timestamp using a `strptime` layout

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
