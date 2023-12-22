## `regex_parser` operator

The `regex_parser` operator parses the string-type field selected by `parse_from` with the given regular expression pattern.

#### Regex Syntax

This operator makes use of [Go regular expression](https://github.com/google/re2/wiki/Syntax). When writing a regex, consider using a tool such as [regex101](https://regex101.com/?flavor=golang).

### Configuration Fields

| Field         | Default          | Description |
| ---           | ---              | ---         |
| `id`          | `regex_parser`   | A unique identifier for the operator. |
| `output`      | Next in pipeline | The connected operator(s) that will receive all outbound entries. |
| `regex`       | required         | A [Go regular expression](https://github.com/google/re2/wiki/Syntax). The named capture groups will be extracted as fields in the parsed body. |
| `parse_from`  | `body`           | The [field](../types/field.md) from which the value will be parsed. |
| `parse_to`    | `attributes`     | The [field](../types/field.md) to which the value will be parsed. |
| `on_error`    | `send`           | The behavior of the operator if it encounters an error. See [on_error](../types/on_error.md). |
| `if`          |                  | An [expression](../types/expression.md) that, when set, will be evaluated to determine whether this operator should be used for the given entry. This allows you to do easy conditional parsing without branching logic with routers. |
| `timestamp`   | `nil`            | An optional [timestamp](../types/timestamp.md) block which will parse a timestamp field before passing the entry to the output operator. |
| `severity`    | `nil`            | An optional [severity](../types/severity.md) block which will parse a severity field before passing the entry to the output operator. |
| `cache`       | `nil`            | An optional cache block. See below for details. |

#### Cache configuration

Regular expression matching results can be cached. This is useful when we need to parse a relatively small set of values repeatedly.
Parsing file paths is a common use case.

The size of the cache is configurable, and it uses a FIFO replacement policy. It also includes athrottling mechanism, which will prevent more than 10% of the maximum size to be replaced within a 5 second interval.

Setting the size to 0 will disable the cache. This is the default.

| Field        | Default  | Description |
| ---          | ---      | ---         |
| `size`       | 0        | The maximum size of the cache. |

### Example Configurations


#### Parse the field `message` with a regular expression

Configuration:
```yaml
- type: regex_parser
  parse_from: body.message
  regex: '^Host=(?P<host>[^,]+), Type=(?P<type>.*)$'
```

<table>
<tr><td> Input body </td> <td> Output body </td></tr>
<tr>
<td>

```json
{
  "timestamp": "",
  "body": {
    "message": "Host=127.0.0.1, Type=HTTP"
  }
}
```

</td>
<td>

```json
{
  "timestamp": "",
  "body": {
    "message": "Host=127.0.0.1, Type=HTTP"
  },
  "attributes": {
    "host": "127.0.0.1",
    "type": "HTTP"
  }
}
```

</td>
</tr>
</table>

#### Parse the body with a regular expression and also parse the timestamp

Configuration:
```yaml
- type: regex_parser
  regex: '^Time=(?P<timestamp_field>\d{4}-\d{2}-\d{2}), Host=(?P<host>[^,]+), Type=(?P<type>.*)$'
  timestamp:
    parse_from: body.timestamp_field
    layout_type: strptime
    layout: '%Y-%m-%d'
```

<table>
<tr><td> Input body </td> <td> Output body </td></tr>
<tr>
<td>

```json
{
  "timestamp": "",
  "body": "Time=2020-01-31, Host=127.0.0.1, Type=HTTP"
}
```

</td>
<td>

```json
{
  "timestamp": "2020-01-31T00:00:00-00:00",
  "body": "Time=2020-01-31, Host=127.0.0.1, Type=HTTP"
  "attributes": {
    "host": "127.0.0.1",
    "type": "HTTP"
  }
}
```

</td>
</tr>
</table>

#### Parse the message field only if "type" is "hostname"

Configuration:
```yaml
- type: regex_parser
  regex: '^Host=(?<host>)$'
  parse_from: body.message
  if: 'body.type == "hostname"'
```

<table>
<tr><td> Input body </td> <td> Output body </td></tr>
<tr>
<td>

```json
{
  "body": {
    "message": "Host=testhost",
    "type": "hostname"
  }
}
```

</td>
<td>

```json
{
  "body": {
    "message": "Host=testhost",
    "type": "hostname"
  },
  "attributes": {
    "host": "testhost"
  }
}
```

</td>
</tr>

<tr>
<td>

```json
{
  "body": {
    "message": "Key=value",
    "type": "keypair"
  }
}
```

</td>
<td>

```json
{
  "body": {
    "message": "Key=value",
    "type": "keypair"
  }
}
```

</td>
</tr>
</table>
