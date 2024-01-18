## `headerless_jarray_parser` operator

The `headerless_jarray_parser` operator parses the string-type field selected by `parse_from` with the given header values.
A JArray string (or a json array string) is a string that represents a JSON array. A JSON array is a type of data structure that is used to store data in a structured way. It consists of an ordered list of values that can be either strings, numbers, objects, or even other arrays.
#### Examples:
a simple Jarray string with strictly strings in it:
```
"[\"Ford\", \"BMW\", \"Fiat\"]"
```

Jarray after parsing:
```json
["Ford", "BMW", "Fiat"]
```

a more complex Jarray string with different types in it without nested objects:
```
"[\"Hello\", 42, true, null]"
```

Jarray after parsing:
```json
["Hello", 42, true, null]
```

a more complex Jarray string with different types in it with nested objects:
```
"[\"Hello\", 42, {\"name\": \"Alice\", \"age\": 25}, [1, 2, 3], true, null]"
```

Jarray after parsing:
```json
["Hello", 42, {"name": "Alice", "age": 25}, [1, 2, 3], true, null]
```

Notice that for this example, the current parser will parse every nested object as a string and so the result is actually this - 
```json
["Hello", 42, "{\"name\": \"Alice\", \"age\": 25}", "[1, 2, 3]", true, null]
```

More information on json arrays can be found [here](https://json-schema.org/understanding-json-schema/reference/array)

#### Adding headers:
This parser can parse such headerless json array strings and match headers to the array's fields. Examples can be seen under Example Configurations section


### Configuration Fields

| Field              | Default                                  | Description                                                                                                                                       |
|--------------------|------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------|
| `id`               | `headerless_jarray_parser`                             | A unique identifier for the operator.                                                                                                             |
| `output`           | Next in pipeline                         | The connected operator(s) that will receive all outbound entries.                                                                                 |
| `header`           | required when `header_attribute` not set | A string of delimited field names                                                                                                                 |
| `header_attribute` | required when `header` not set           | An attribute name to read the header field from, to support dynamic field names                                                                   |
| `header_delimiter` | value of `delimiter`                     | A character that will be used as a delimiter for headers. Values `\r` and `\n` cannot be used as a delimiter.                                       |
| `parse_from`       | `body`                                   | The [field](../types/field.md) from which the value will be parsed.                                                                               |
| `parse_to`         | `attributes`                             | The [field](../types/field.md) to which the value will be parsed.                                                                                 |
| `on_error`         | `send`                                   | The behavior of the operator if it encounters an error. See [on_error](../types/on_error.md).                                                     |
| `timestamp`        | `nil`                                    | An optional [timestamp](../types/timestamp.md) block which will parse a timestamp field before passing the entry to the output operator.          |
| `severity`         | `nil`                                    | An optional [severity](../types/severity.md) block which will parse a severity field before passing the entry to the output operator.             |

### Embedded Operations

The `headerless_jarray` can be configured to embed certain operations such as timestamp and severity parsing. For more information, see [complex parsers](../types/parsers.md#complex-parsers).

### Example Configurations

#### Parse the field `body` with a headerless jarray parser into attributes

Configuration:

```yaml
- type: headerless_jarray_parser
  parse_from: body
  parse_to: attributes
  header: id,severity,message,isExample
```

<table>
<tr><td> Input Entry </td> <td> Output Entry </td></tr>
<tr>
<td>

```json
{
  "body": "[1,\"debug\",\"Debug Message\", true]"
}
```

</td>
<td>

```json
{
  "attributes": {
    "id": 1,
    "severity": "debug",
    "message": "Debug Message",
    "isExample": true
  }
}
```

</td>
</tr>
</table>

#### Parse the field `body` with a headerless jarray parser into attributes using | as header delimiter

Configuration:

```yaml
- type: headerless_jarray_parser
  parse_from: body
  parse_to: attributes
  header: id|severity|message|isExample
  header_delimiter: "|"
```

<table>
<tr><td> Input Entry </td> <td> Output Entry </td></tr>
<tr>
<td>

```json
{
  "body": "[1,\"debug\",\"Debug Message\", true]"
}
```

</td>
<td>

```json
{
  "attributes": {
    "id": 1,
    "severity": "debug",
    "message": "Debug Message",
    "isExample": true
  }
}
```

</td>
</tr>
</table>

#### Parse the field `message` with a jarray parser

Configuration:

```yaml
- type: headerless_jarray_parser
  header: id,severity,message,isExample
```

<table>
<tr><td> Input Entry </td> <td> Output Entry </td></tr>
<tr>
<td>

```json
{
  "body": {
    "message":"[1,\"debug\",\"Debug Message\", true]"
  }
}
```

</td>
<td>

```json
{
  "body": {
    "id": 1,
    "severity": "debug",
    "message": "Debug Message",
    "isExample": true
  }
}
```

</td>
</tr>
</table>