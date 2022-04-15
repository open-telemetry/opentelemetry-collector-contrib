## `json_parser` operator

The `json_parser` operator parses the string-type field selected by `parse_from` as JSON.

### Configuration Fields

| Field         | Default          | Description |
| ---           | ---              | ---         |
| `id`          | `json_parser`    | A unique identifier for the operator. |
| `output`      | Next in pipeline | The connected operator(s) that will receive all outbound entries. |
| `parse_from`  | `body`           | The [field](/docs/types/field.md) from which the value will be parsed. |
| `parse_to`    | `body`           | The [field](/docs/types/field.md) to which the value will be parsed. |
| `on_error`    | `send`           | The behavior of the operator if it encounters an error. See [on_error](/docs/types/on_error.md). |
| `if`          |                  | An [expression](/docs/types/expression.md) that, when set, will be evaluated to determine whether this operator should be used for the given entry. This allows you to do easy conditional parsing without branching logic with routers. |
| `timestamp`   | `nil`            | An optional [timestamp](/docs/types/timestamp.md) block which will parse a timestamp field before passing the entry to the output operator. |
| `severity`    | `nil`            | An optional [severity](/docs/types/severity.md) block which will parse a severity field before passing the entry to the output operator. |


### Example Configurations


#### Parse the field `message` as JSON

Configuration:
```yaml
- type: json_parser
  parse_from: body.message
```

<table>
<tr><td> Input body </td> <td> Output body</td></tr>
<tr>
<td>

```json
{
  "timestamp": "",
  "body": {
    "message": "{\"key\": \"val\"}"
  }
}
```

</td>
<td>

```json
{
  "timestamp": "",
  "body": {
    "key": "val"
  }
}
```

</td>
</tr>
</table>

#### Parse the field `message` as JSON, and parse the timestamp

Configuration:
```yaml
- type: json_parser
  parse_from: body.message
  timestamp:
    parse_from: body.seconds_since_epoch
    layout_type: epoch
    layout: s
```

<table>
<tr><td> Input body </td> <td> Output body </td></tr>
<tr>
<td>

```json
{
  "timestamp": "",
  "body": {
    "message": "{\"key\": \"val\", \"seconds_since_epoch\": 1136214245}"
  }
}
```

</td>
<td>

```json
{
  "timestamp": "2006-01-02T15:04:05-07:00",
  "body": {
    "key": "val"
  }
}
```

</td>
</tr>
</table>

#### Parse the body only if it starts and ends with brackets

Configuration:
```yaml
- type: json_parser
  if: '$matches "^{.*}$"'
```

<table>
<tr><td> Input body </td> <td> Output body </td></tr>
<tr>
<td>

```json
{
  "body": "{\"key\": \"val\"}"
}
```

</td>
<td>

```json
{
  "body": {
    "key": "val"
  }
}
```

</td>
</tr>

<tr>
<td>

```json
{
  "body": "notjson"
}
```

</td>
<td>

```json
{
  "body": "notjson"
}
```

</td>
</tr>
</table>
