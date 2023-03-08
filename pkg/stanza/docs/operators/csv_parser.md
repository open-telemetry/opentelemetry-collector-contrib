## `csv_parser` operator

The `csv_parser` operator parses the string-type field selected by `parse_from` with the given header values.

### Configuration Fields

| Field              | Default                                  | Description                                                                                                                                       |
|--------------------|------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------|
| `id`               | `csv_parser`                             | A unique identifier for the operator.                                                                                                             |
| `output`           | Next in pipeline                         | The connected operator(s) that will receive all outbound entries.                                                                                 |
| `header`           | required when `header_attribute` not set | A string of delimited field names                                                                                                                 |
| `header_attribute` | required when `header` not set           | An attribute name to read the header field from, to support dynamic field names                                                                   |
| `delimiter`        | `,`                                      | A character that will be used as a delimiter. Values `\r` and `\n` cannot be used as a delimiter.                                                 |
| `header_delimiter` | value of `delimiter`                     | A character that will be used as a delimiter for headers. Values `\r` and `\n` cannot be used as a delimiter.                                       |
| `lazy_quotes`      | `false`                                  | If true, a quote may appear in an unquoted field and a non-doubled quote may appear in a quoted field. Cannot be true if `ignore_quotes` is true. |
| `ignore_quotes`    | `false`                                  | If true, all quotes are ignored, and fields are simply split on the delimiter. Cannot be true if `lazy_quotes` is true.                           |
| `parse_from`       | `body`                                   | The [field](../types/field.md) from which the value will be parsed.                                                                               |
| `parse_to`         | `attributes`                             | The [field](../types/field.md) to which the value will be parsed.                                                                                 |
| `on_error`         | `send`                                   | The behavior of the operator if it encounters an error. See [on_error](../types/on_error.md).                                                     |
| `timestamp`        | `nil`                                    | An optional [timestamp](../types/timestamp.md) block which will parse a timestamp field before passing the entry to the output operator.          |
| `severity`         | `nil`                                    | An optional [severity](../types/severity.md) block which will parse a severity field before passing the entry to the output operator.             |

### Embedded Operations

The `csv_parser` can be configured to embed certain operations such as timestamp and severity parsing. For more information, see [complex parsers](../types/parsers.md#complex-parsers).

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
