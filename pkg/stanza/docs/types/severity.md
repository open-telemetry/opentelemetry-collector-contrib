## Severity Parsing

Severity is represented as a number from 1 to 24. The meaning of these severity levels are defined in the [OpenTelemetry Logs Data Model](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/logs/data-model.md#field-severitynumber).

> Note: A `default` severity level is also supported, and is used when a value cannot be mapped to any other level.

### `severity` parsing parameters

Parser operators can parse a severity and attach the resulting value to a log entry.

| Field            | Default   | Description |
| ---              | ---       | ---         |
| `parse_from`     | required  | The [field](../types/field.md) from which the value will be parsed. |
| `preset`         | `default` | A predefined set of values that should be interpretted at specific severity levels. |
| `mapping`        |           | A custom set of values that should be interpretted at designated severity levels. |
| `overwrite_text` | `false`   | If `true`, the severity text will be set to the [recommeneded short name](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/logs/data-model.md#displaying-severity) corresponding to the severity number. |

Note that by default the severity _text_ will be set to the original value which was interpreted into a severity number. In order to set the severity text to a standard short name (e.g. `ERROR`, `INFO3`, etc.), set `overwrite_text` to `true`.

### How severity `mapping` works

Severity parsing behavior is defined in a config file using a severity `mapping`. The general structure of the `mapping` is as follows:

```yaml
...
  mapping:
    severity_alias: value | list of values | range | special
    severity_alias: value | list of values | range | special
```

The following aliases are used to represent the possible severity levels:

| Severity Number | Alias     |
| ---             | ---       |
|        0        | `default` |
|        1        | `trace`   |
|        2        | `trace2`  |
|        3        | `trace3`  |
|        4        | `trace4`  |
|        5        | `debug`   |
|        6        | `debug2`  |
|        7        | `debug3`  |
|        8        | `debug4`  |
|        9        | `info`    |
|        10       | `info2`   |
|        11       | `info3`   |
|        12       | `info4`   |
|        13       | `warn`    |
|        14       | `warn2`   |
|        15       | `warn3`   |
|        16       | `warn4`   |
|        17       | `error`   |
|        18       | `error2`  |
|        19       | `error3`  |
|        20       | `error4`  |
|        21       | `fatal`   |
|        22       | `fatal2`  |
|        23       | `fatal3`  |
|        24       | `fatal4`  |

The following example illustrates many of the ways in which mapping can configured:
```yaml
...
  mapping:

    # single value to be parsed as "error"
    error: oops

    # list of values to be parsed as "warn"
    warn:
      - hey!
      - YSK

    # range of values to be parsed as "info"
    info:
      - min: 300
        max: 399

    # special value representing the range 200-299, to be parsed as "debug"
    debug: 2xx

    # single value to be parsed as a "info3"
    info3: medium

    # mix and match the above concepts
    fatal:
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
    trace2: trace2
    trace3: trace3
    trace4: trace4
    debug: debug
    debug2: debug2
    debug3: debug3
    debug4: debug4
    info: info
    info2: info2
    info3: info3
    info4: info4
    warn: warn
    warn2: warn2
    warn3: warn3
    warn4: warn4
    error:
      - error
      - 404
    error2: error2
    error3: error3
    error4: error4
    fatal: fatal
    fatal2: fatal2
    fatal3: fatal3
    fatal4: fatal4
```

<sub>Additional built-in presets coming soon</sub>


### How to use severity parsing

All parser operators, such as [`regex_parser`](../operators/regex_parser.md) support these fields inside of a `severity` block.

If a severity block is specified, the parser operator will perform the severity parsing _after_ performing its other parsing actions, but _before_ passing the entry to the specified output operator.

```yaml
- type: regex_parser
  regexp: '^StatusCode=(?P<severity_field>\d{3}), Host=(?P<host>[^,]+)'
  severity:
    parse_from: body.severity_field
    mapping:
      warn: 5xx
      error: 4xx
      info: 3xx
      debug: 2xx
```

---

As a special case, the [`severity_parser`](../operators/severity_parser.md) operator supports these fields inline. This is because severity parsing is the primary purpose of the operator.
```yaml
- type: severity_parser
  parse_from: body.severity_field
  mapping:
    warn: 5xx
    error: 4xx
    info: 3xx
    debug: 2xx
```

### Example Configurations

#### Parse a severity from a standard value

Configuration:
```yaml
- type: severity_parser
  parse_from: body.severity_field
```

Note that the default `preset` is in place, and no additional values have been specified.

<table>
<tr><td> Input entry </td> <td> Output entry </td></tr>
<tr>
<td>

```json
{
  "severity": "default",
  "body": {
    "severity_field": "ERROR"
  }
}
```

</td>
<td>

```json
{
  "severity": "error",
  "body": {}
}
```

</td>
</tr>
</table>

#### Parse a severity from a non-standard value

Configuration:
```yaml
- type: severity_parser
  parse_from: body.severity_field
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
  "severity": "default",
  "body": {
    "severity_field": "nooo!"
  }
}
```

</td>
<td>

```json
{
  "severity": "error",
  "body": {}
}
```

</td>
</tr>
<tr>
<td>

```json
{
  "severity": "default",
  "body": {
    "severity_field": "ERROR"
  }
}
```

</td>
<td>

```json
{
  "severity": "error",
  "body": {}
}
```

</td>
</tr>
</table>

#### Parse a severity from any of several non-standard values

Configuration:
```yaml
- type: severity_parser
  parse_from: body.severity_field
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
  "severity": "default",
  "body": {
    "severity_field": "nooo!"
  }
}
```

</td>
<td>

```json
{
  "severity": "error",
  "body": {}
}
```

</td>
</tr>
<tr>
<td>

```json
{
  "severity": "default",
  "body": {
    "severity_field": "nooooooo"
  }
}
```

</td>
<td>

```json
{
  "severity": "error",
  "body": {}
}
```

</td>
</tr>
<tr>
<td>

```json
{
  "severity": "default",
  "body": {
    "severity_field": "hey"
  }
}
```

</td>
<td>

```json
{
  "severity": "info",
  "body": {}
}
```

</td>
</tr>
<tr>
<td>

```json
{
  "severity": "default",
  "body": {
    "severity_field": 1234
  }
}
```

</td>
<td>

```json
{
  "severity": "debug",
  "body": {}
}
```

</td>
</tr>
<tr>
<td>

```json
{
  "severity": "default",
  "body": {
    "severity_field": "unknown"
  }
}
```

</td>
<td>

```json
{
  "severity": "default",
  "body": {}
}
```

</td>
</tr>
</table>

#### Parse a severity from a range of values

Configuration:
```yaml
- type: severity_parser
  parse_from: body.severity_field
  mapping:
    error:
      - min: 1
        max: 5
    fatal:
      - min: 6
        max: 10
```

<table>
<tr><td> Input entry </td> <td> Output entry </td></tr>
<tr>
<td>

```json
{
  "severity": "default",
  "body": {
    "severity_field": 3
  }
}
```

</td>
<td>

```json
{
  "severity": "error",
  "body": {}
}
```

</td>
</tr>
<tr>
<td>

```json
{
  "severity": "default",
  "body": {
    "severity_field": 9
  }
}
```

</td>
<td>

```json
{
  "severity": "fatal",
  "body": {}
}
```

</td>
</tr>
<tr>
<td>

```json
{
  "severity": "default",
  "body": {
    "severity_field": 12
  }
}
```

</td>
<td>

```json
{
  "severity": "default",
  "body": {}
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
  parse_from: body.severity_field
  mapping:
    warn: 5xx
    error: 4xx
    info: 3xx
    debug: 2xx
```

Equivalent Configuration:
```yaml
- id: my_severity_parser
  type: severity_parser
  parse_from: body.severity_field
  mapping:
    warn:
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
  "severity": "default",
  "body": {
    "severity_field": 302
  }
}
```

</td>
<td>

```json
{
  "severity": "info",
  "body": {}
}
```

</td>
</tr>
<tr>
<td>

```json
{
  "severity": "default",
  "body": {
    "severity_field": 404
  }
}
```

</td>
<td>

```json
{
  "severity": "error",
  "body": {}
}
```

</td>
</tr>
<tr>
<td>

```json
{
  "severity": "default",
  "body": {
    "severity_field": 200
  }
}
```

</td>
<td>

```json
{
  "severity": "debug",
  "body": {}
}
```

</td>
</tr>
</table>

#### Parse a severity from a value without using the default preset

Configuration:
```yaml
- type: severity_parser
  parse_from: body.severity_field
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
  "severity": "default",
  "body": {
    "severity_field": "nooo!"
  }
}
```

</td>
<td>

```json
{
  "severity": "error",
  "body": {}
}
```

</td>
</tr>
<tr>
<td>

```json
{
  "severity": "default",
  "body": {
    "severity_field": "ERROR"
  }
}
```

</td>
<td>

```json
{
  "severity": "default",
  "body": {}
}
```

</td>
</tr>
</table>
