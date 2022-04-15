## `csv_parser` operator

The `csv_parser` operator parses the string-type field selected by `parse_from` with the given header values.

### Configuration Fields

| Field              | Default                                  | Description  |
| ---                | ---                                      | ---          |
| `id`               | `csv_parser`                             | A unique identifier for the operator. |
| `output`           | Next in pipeline                         | The connected operator(s) that will receive all outbound entries. |
| `header`           | required when `header_attribute` not set | A string of delimited field names |
| `header_attribute` | required when `header` not set           | An attribute name to read the header field from, to support dynamic field names |
| `delimiter`        | `,`                                      | A character that will be used as a delimiter. Values `\r` and `\n` cannot be used as a delimiter. |
| `lazy_quotes`      | `false`                                  | If true, a quote may appear in an unquoted field and a non-doubled quote may appear in a quoted field. |
| `parse_from`       | `body`                                   | The [field](/docs/types/field.md) from which the value will be parsed. |
| `parse_to`         | `body`                                   | The [field](/docs/types/field.md) to which the value will be parsed. |
| `preserve_to`      |                                          | Preserves the unparsed value at the specified [field](/docs/types/field.md). |
| `on_error`         | `send`                                   | The behavior of the operator if it encounters an error. See [on_error](/docs/types/on_error.md). |
| `timestamp`        | `nil`                                    | An optional [timestamp](/docs/types/timestamp.md) block which will parse a timestamp field before passing the entry to the output operator. |
| `severity`         | `nil`                                    | An optional [severity](/docs/types/severity.md) block which will parse a severity field before passing the entry to the output operator. |

### Example Configurations

#### Parse the field `message` with a csv parser

Configuration:

```yaml
- type: csv_parser
  header: id,severity,message
```

<table>
<tr><td> Input Entry </td> <td> Output Entry </td></tr>
<tr>
<td>

```json
{
  "body": "1,debug,Debug Message"
}
```

</td>
<td>

```json
{
  "body": {
    "id": "1",
    "severity": "debug",
    "message": "Debug Message"
  }
}
```

</td>
</tr>
</table>

#### Parse the field `message` with a csv parser using tab delimiter

Configuration:

```yaml
- type: csv_parser
  parse_from: body.message
  header: id,severity,message
  delimiter: "\t"
```

<table>
<tr><td> Input Entry </td> <td> Output Entry </td></tr>
<tr>
<td>

```json
{
  "body": {
    "message": "1 debug Debug Message"
  }
}
```

</td>
<td>

```json
{
  "body": {
    "id": "1",
    "severity": "debug",
    "message": "Debug Message"
  }
}
```

</td>
</tr>
</table>

#### Parse the field `message` with csv parser and also parse the timestamp

Configuration:

```yaml
- type: csv_parser
  header: 'timestamp_field,severity,message'
  timestamp:
    parse_from: body.timestamp_field
    layout_type: strptime
    layout: '%Y-%m-%d'
```

<table>
<tr><td> Input Entry </td> <td> Output Entry </td></tr>
<tr>
<td>

```json
{
  "timestamp": "",
  "body": {
    "message": "2021-03-17,debug,Debug Message"
  }
}
```

</td>
<td>

```json
{
  "timestamp": "2021-03-17T00:00:00-00:00",
  "body": {
    "severity": "debug",
    "message": "Debug Message"
  }
}
```

</td>
</tr>
</table>

#### Parse the field `message` using dynamic field names

Dynamic field names can be had when leveraging file_input's `label_regex`.

Configuration:

```yaml
- type: file_input
  include:
  - ./dynamic.log
  start_at: beginning
  label_regex: '^#(?P<key>.*?): (?P<value>.*)'

- type: csv_parser
  delimiter: ","
  header_attribute: Fields
```

Input File:

```
#Fields: "id,severity,message"
1,debug,Hello
```

<table>
<tr><td> Input record </td> <td> Output record </td></tr>
<tr>
<td>

Entry (from file_input):

```json
{
  "timestamp": "",
  "labels": {
    "fields": "id,severity,message"
  },
  "record": {
    "message": "1,debug,Hello"
  }
}
```

</td>
<td>

```json
{
  "timestamp": "",
  "labels": {
    "fields": "id,severity,message"
  },
  "record": {
    "id": "1",
    "severity": "debug",
    "message": "Hello"
  }
}
```

</td>
</tr>
</table>
