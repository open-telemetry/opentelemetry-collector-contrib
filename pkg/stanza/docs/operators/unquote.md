## `unquote` operator

The `unquote` operator unquotes a string.

### Configuration Fields

| Field      | Default          | Description |
| ---        | ---              | ---         |
| `id`       | `unquote`        | A unique identifier for the operator. |
| `output`   | Next in pipeline | The connected operator(s) that will receive all outbound entries. |
| `field`    | required         | The [field](../types/field.md) to unquote. Must be a string. |
| `on_error` | `send`           | The behavior of the operator if it encounters an error. See [on_error](../types/on_error.md). |
| `if`       |                  | An [expression](../types/expression.md) that, when set, will be evaluated to determine whether this operator should be used for the given entry. This allows you to do easy conditional parsing without branching logic with routers. |

The operator applies the [strconv.Unquote]() function to the specified field. This can only be applied to strings that are double quoted ("\\\"foo\\\""), back quoted ("\`bar\`"), or to single characters that are single quoted ("'v'")

### Example Configurations:

<hr>

Unquote the body
```yaml
- type: remove
  field: body
```

<table>
<tr><td> Input Entry </td> <td> Output Entry </td></tr>
<tr>
<td>

```json
{
  "resource": { },
  "attributes": { },
  "body": "\"hello\""
}
```

</td>
<td>

```json
{
  "resource": { },
  "attributes": { },
  "body": "hello"
}
```

</td>
</tr>
</table>

<hr>

Unquote an attribute
```yaml
- type: unquote
  field: attributes.foo
```

<table>
<tr><td> Input Entry </td> <td> Output Entry </td></tr>
<tr>
<td>

```json
{
  "resource": { },
  "attributes": {
    "foo": "`bar`"
  },
}
```

</td>
<td>

```json
{
  "resource": { },
  "attributes": {
    "foo": "bar"
  },
}
```

</td>
</tr>
</table>
