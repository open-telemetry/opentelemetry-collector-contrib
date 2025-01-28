## `strip_ansi_escape_codes` operator

The `strip_ansi_escape_codes` operator removes ANSI escape sequences (such as color codes) from a string.

### Configuration Fields

| Field      | Default                   | Description |
| ---        | ---                       | ---         |
| `id`       | `strip_ansi_escape_codes` | A unique identifier for the operator. |
| `output`   | Next in pipeline          | The connected operator(s) that will receive all outbound entries. |
| `field`    | required                  | The [field](../types/field.md) to strip. Must be a string. |
| `on_error` | `send`                    | The behavior of the operator if it encounters an error. See [on_error](../types/on_error.md). |
| `if`       |                           | An [expression](../types/expression.md) that, when set, will be evaluated to determine whether this operator should be used for the given entry. This allows you to do easy conditional parsing without branching logic with routers. |

Right now the operator only removes the ANSI "Control Sequence Introducer (CSI)" escape codes starting with `ESC [`.

### Example Configurations:

<hr>

Remove all color escape codes from the body
```yaml
- type: strip_ansi_escape_codes
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
  "body": "\x1b[31mred\x1b[0m"
}
```

</td>
<td>

```json
{
  "resource": { },
  "attributes": { },
  "body": "red"
}
```

</td>
</tr>
</table>
