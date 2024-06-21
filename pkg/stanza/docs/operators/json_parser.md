## `json_parser` operator

The `json_parser` operator parses the string-type field selected by `parse_from` as JSON.

### Configuration Fields

| Field             | Default          | Description |
| ---               | ---              | ---         |
| `id`              | `json_parser`    | A unique identifier for the operator. |
| `output`          | Next in pipeline | The connected operator(s) that will receive all outbound entries. |
| `parse_from`      | `body`           | The [field](../types/field.md) from which the value will be parsed. |
| `parse_to`        | `attributes`     | The [field](../types/field.md) to which the value will be parsed. |
| `on_error`        | `send`           | The behavior of the operator if it encounters an error. See [on_error](../types/on_error.md). |
| `if`              |                  | An [expression](../types/expression.md) that, when set, will be evaluated to determine whether this operator should be used for the given entry. This allows you to do easy conditional parsing without branching logic with routers. |
| `timestamp`       | `nil`            | An optional [timestamp](../types/timestamp.md) block which will parse a timestamp field before passing the entry to the output operator. |
| `severity`        | `nil`            | An optional [severity](../types/severity.md) block which will parse a severity field before passing the entry to the output operator. |
| `jsontier_config` | `nil`            | An optional jsontier config block. See below for details. |

### Embedded Operations

The `json_parser` can be configured to embed certain operations such as timestamp and severity parsing. For more information, see [complex parsers](../types/parsers.md#complex-parsers).

### `jsoniter_config` Configuration

The `json_parser` operator uses the [json-iterator](https://github.com/json-iterator/go) as the underlying json parser, the default config is [ConfigFastest](https://pkg.go.dev/github.com/json-iterator/go#pkg-variables).

In additional, this `jsoniter_config` block allows you to configure the json parser with a custom configuration. Here are available fields that map to the corresponding fields in [json-iterator Config](https://pkg.go.dev/github.com/json-iterator/go#Config):

| Field                                | Default | Description                                        |
|--------------------------------------|---------|----------------------------------------------------|
| `indention_step`                     | 0       | json-iterator.Config.IndentionStep                 |
| `marshal_float_with_6_digits`        | `false` | json-iterator.Config.MarshalFloatWith6Digits       |
| `escape_html`                        | `false` | json-iterator.Config.EscapeHTML                    |
| `sort_map_keys`                      | `false` | json-iterator.Config.SortMapKeys                   |
| `use_number`                         | `false` | json-iterator.Config.UseNumber                     |
| `disallow_unknown_fields`            | `false` | json-iterator.Config.DisallowUnknownFields         |
| `tag_key`                            | ``      | json-iterator.Config.TagKey                        |
| `only_tagged_field`                  | `false` | json-iterator.Config.OnlyTaggedField               |
| `validate_json_raw_message`          | `false` | json-iterator.Config.ValidateJsonRawMessage        |
| `object_field_must_be_simple_string` | `false` | json-iterator.Config.ObjectFieldMustBeSimpleString |
| `case_sensitive`                     | `false` | json-iterator.Config.CaseSensitive                 |

numbers like `int` and `float` are parsed as `float64` by default, when `use_number` is enabled, numbers are parsed as `json.Number` and then coverted to `int64` or `float64` based on the value.

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
  if: 'body matches "^{.*}$"'
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
