## `csv_parser` operator

The `csv_parser` operator parses the string-type field selected by `parse_from` with the given header values.
 
### Configuration Fields 

| Field         | Default          | Description  |
| ---           | ---              | ---          |
| `id`          | `csv_parser`     | A unique identifier for the operator. |
| `output`      | Next in pipeline | The connected operator(s) that will receive all outbound entries. |
| `header`      | required         | A string of delimited field names. The values in the delimited header will be used as keys. |
| `delimiter`   | `,`              | A character that will be used as a delimiter. Values `\r` and `\n` cannot be used as a delimiter. |
| `parse_from`  | $body            | The [field](/docs/types/field.md) from which the value will be parsed. |
| `parse_to`    | $body            | The [field](/docs/types/field.md) to which the value will be parsed. |
| `preserve_to` |                  | Preserves the unparsed value at the specified [field](/docs/types/field.md). |
| `on_error`    | `send`           | The behavior of the operator if it encounters an error. See [on_error](/docs/types/on_error.md). |
| `timestamp`   | `nil`            | An optional [timestamp](/docs/types/timestamp.md) block which will parse a timestamp field before passing the entry to the output operator. |
| `severity`    | `nil`            | An optional [severity](/docs/types/severity.md) block which will parse a severity field before passing the entry to the output operator. |

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
  parse_from: message
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
    parse_from: timestamp_field
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