## Severity Parsing

`stanza` uses a flexible severity parsing system based on the integers 0 to 100. Standard severities are provided at multiples of 10.

This severity system allows each output operator to interpret the values 0 to 100 as appropriate for the corresponding backend.

The following named severity levels are supported.

| Severity    | Numeric Value | Alias         |
| ---         | ---           | ---           |
| Default     |        0      | `default`     |
| Trace       |       10      | `trace`       |
| Debug       |       20      | `debug`       |
| Info        |       30      | `info`        |
| Notice      |       40      | `notice`      |
| Warning     |       50      | `warning`     |
| Error       |       60      | `error`       |
| Critical    |       70      | `critical`    |
| Alert       |       80      | `alert`       |
| Emergency   |       90      | `emergency`   |
| Catastrophe |      100      | `catastrophe` |


### `severity` parsing parameters

Parser operators can parse a severity and attach the resulting value to a log entry.

| Field          | Default   | Description                                                                        |
| ---            | ---       | ---                                                                                |
| `parse_from`   | required  | A [field](/docs/types/field.md) that indicates the field to be parsed as JSON      |
| `preserve_to` |                  | Preserves the unparsed value at the specified [field](/docs/types/field.md)  |
| `preset`       | `default` | A predefined set of values that should be interpretted at specific severity levels |
| `mapping`      |           | A custom set of values that should be interpretted at designated severity levels   |


### How severity `mapping` works

Severity parsing behavior is defined in a config file using a severity `mapping`. The general structure of the `mapping` is as follows:

```yaml
...
  mapping:
    severity_as_int_or_alias: value | list of values | range | special
    severity_as_int_or_alias: value | list of values | range | special
```

The following example illustrates many of the ways in which mapping can configured:
```yaml
...
  mapping:

    # single value to be parsed as "error"
    error: oops

    # list of values to be parsed as "warning"
    warning:
      - hey!
      - YSK

    # range of values to be parsed as "info"
    info:
      - min: 300
        max: 399

    # special value representing the range 200-299, to be parsed as "debug"
    debug: 2xx

    # single value to be parsed as a custom level of 36
    36: medium

    # mix and match the above concepts
    95:
      - really serious
      - min: 9001
        max: 9050
      - 5xx
```

### How to simplify configuration with a `preset`

A `preset` can reduce the amount of configuration needed in the `mapping` structure by initializing the severity mapping with common values. Values specified in the more verbose `mapping` structure will then be added to the severity map.

By default, a common `preset` is used. Alternately, `preset: none` can be specified to start with an empty mapping.

The following configurations are equivalent:

```yaml
...
  mapping:
    error: 404
```

```yaml
...
  preset: default
  mapping:
    error: 404
```

```yaml
...
  preset: none
  mapping:
    trace: trace
    debug: debug
    info: info
    notice: notice
    warning:
      - warning
      - warn
    error:
      - error
      - err
      - 404
    critical:
      - critical
      - crit
    alert: alert
    emergency: emergency
    catastrophe: catastrophe
```

<sub>Additional built-in presets coming soon</sub>


### How to use severity parsing

All parser operators, such as [`regex_parser`](/docs/operators/regex_parser.md) support these fields inside of a `severity` block.

If a severity block is specified, the parser operator will perform the severity parsing _after_ performing its other parsing actions, but _before_ passing the entry to the specified output operator.

```yaml
- type: regex_parser
  regexp: '^StatusCode=(?P<severity_field>\d{3}), Host=(?P<host>[^,]+)'
  severity:
    parse_from: severity_field
    mapping:
      critical: 5xx
      error: 4xx
      info: 3xx
      debug: 2xx
```

---

As a special case, the [`severity_parser`](/docs/operators/severity_parser.md) operator supports these fields inline. This is because severity parsing is the primary purpose of the operator.
```yaml
- type: severity_parser
  parse_from: severity_field
  mapping:
    critical: 5xx
    error: 4xx
    info: 3xx
    debug: 2xx
```

### Example Configurations

#### Parse a severity from a standard value

Configuration:
```yaml
- type: severity_parser
  parse_from: severity_field
```

Note that the default `preset` is in place, and no additional values have been specified.

<table>
<tr><td> Input entry </td> <td> Output entry </td></tr>
<tr>
<td>

```json
{
  "severity": 0,
  "record": {
    "severity_field": "ERROR"
  }
}
```

</td>
<td>

```json
{
  "severity": 60,
  "record": {}
}
```

</td>
</tr>
</table>

#### Parse a severity from a non-standard value

Configuration:
```yaml
- type: severity_parser
  parse_from: severity_field
  mapping:
    error: nooo!
```

Note that the default `preset` is in place, and one additional values has been specified.

<table>
<tr><td> Input entry </td> <td> Output entry </td></tr>
<tr>
<td>

```json
{
  "severity": 0,
  "record": {
    "severity_field": "nooo!"
  }
}
```

</td>
<td>

```json
{
  "severity": 60,
  "record": {}
}
```

</td>
</tr>
<tr>
<td>

```json
{
  "severity": 0,
  "record": {
    "severity_field": "ERROR"
  }
}
```

</td>
<td>

```json
{
  "severity": 60,
  "record": {}
}
```

</td>
</tr>
</table>

#### Parse a severity from any of several non-standard values

Configuration:
```yaml
- type: severity_parser
  parse_from: severity_field
  mapping:
    error:
      - nooo!
      - nooooooo
    info: HEY
    debug: 1234
```

<table>
<tr><td> Input entry </td> <td> Output entry </td></tr>
<tr>
<td>

```json
{
  "severity": 0,
  "record": {
    "severity_field": "nooo!"
  }
}
```

</td>
<td>

```json
{
  "severity": 60,
  "record": {}
}
```

</td>
</tr>
<tr>
<td>

```json
{
  "severity": 0,
  "record": {
    "severity_field": "nooooooo"
  }
}
```

</td>
<td>

```json
{
  "severity": 60,
  "record": {}
}
```

</td>
</tr>
<tr>
<td>

```json
{
  "severity": 0,
  "record": {
    "severity_field": "hey"
  }
}
```

</td>
<td>

```json
{
  "severity": 30,
  "record": {}
}
```

</td>
</tr>
<tr>
<td>

```json
{
  "severity": 0,
  "record": {
    "severity_field": 1234
  }
}
```

</td>
<td>

```json
{
  "severity": 20,
  "record": {}
}
```

</td>
</tr>
<tr>
<td>

```json
{
  "severity": 0,
  "record": {
    "severity_field": "unknown"
  }
}
```

</td>
<td>

```json
{
  "severity": 0,
  "record": {}
}
```

</td>
</tr>
</table>

#### Parse a severity from a range of values

Configuration:
```yaml
- type: severity_parser
  parse_from: severity_field
  mapping:
    error:
      - min: 1
        max: 5
    alert:
      - min: 6
        max: 10
```

<table>
<tr><td> Input entry </td> <td> Output entry </td></tr>
<tr>
<td>

```json
{
  "severity": 0,
  "record": {
    "severity_field": 3
  }
}
```

</td>
<td>

```json
{
  "severity": 60,
  "record": {}
}
```

</td>
</tr>
<tr>
<td>

```json
{
  "severity": 0,
  "record": {
    "severity_field": 9
  }
}
```

</td>
<td>

```json
{
  "severity": 80,
  "record": {}
}
```

</td>
</tr>
<tr>
<td>

```json
{
  "severity": 0,
  "record": {
    "severity_field": 12
  }
}
```

</td>
<td>

```json
{
  "severity": 0,
  "record": {}
}
```

</td>
</tr>
</table>

#### Parse a severity from a HTTP Status Codes value

Special values are provided to represent http status code ranges.

| Value | Meaning   |
| ---   | ---       |
| 2xx   | 200 - 299 |
| 3xx   | 300 - 399 |
| 4xx   | 400 - 499 |
| 5xx   | 500 - 599 |

Configuration:
```yaml
- type: severity_parser
  parse_from: severity_field
  mapping:
    critical: 5xx
    error: 4xx
    info: 3xx
    debug: 2xx
```

Equivalent Configuration:
```yaml
- id: my_severity_parser
  type: severity_parser
  parse_from: severity_field
  mapping:
    critical:
      - min: 500
        max: 599
    error:
      - min: 400
        max: 499
    info:
      - min: 300
        max: 399
    debug:
      - min: 200
        max: 299
  output: my_next_operator
```

<table>
<tr><td> Input entry </td> <td> Output entry </td></tr>
<tr>
<td>

```json
{
  "severity": 0,
  "record": {
    "severity_field": 302
  }
}
```

</td>
<td>

```json
{
  "severity": 30,
  "record": {}
}
```

</td>
</tr>
<tr>
<td>

```json
{
  "severity": 0,
  "record": {
    "severity_field": 404
  }
}
```

</td>
<td>

```json
{
  "severity": 60,
  "record": {}
}
```

</td>
</tr>
<tr>
<td>

```json
{
  "severity": 0,
  "record": {
    "severity_field": 200
  }
}
```

</td>
<td>

```json
{
  "severity": 20,
  "record": {}
}
```

</td>
</tr>
</table>

#### Parse a severity from a value without using the default preset

Configuration:
```yaml
- type: severity_parser
  parse_from: severity_field
  preset: none
  mapping:
    error: nooo!
```

<table>
<tr><td> Input entry </td> <td> Output entry </td></tr>
<tr>
<td>

```json
{
  "severity": 0,
  "record": {
    "severity_field": "nooo!"
  }
}
```

</td>
<td>

```json
{
  "severity": 60,
  "record": {}
}
```

</td>
</tr>
<tr>
<td>

```json
{
  "severity": 0,
  "record": {
    "severity_field": "ERROR"
  }
}
```

</td>
<td>

```json
{
  "severity": 0,
  "record": {}
}
```

</td>
</tr>
</table>
