## `regex_replace` operator

The `regex_replace` operator parses the string-typed field selected by `field` with the given user-defined or well-known regular expression.
Optionally, it replaces the matched string.

#### Regex Syntax

This operator makes use of [Go regular expression](https://github.com/google/re2/wiki/Syntax). When writing a regex, consider using a tool such as [regex101](https://regex101.com/?flavor=golang).

### Configuration Fields

| Field          | Default                          | Description |
| ---            | ---                              | ---         |
| `id`           | `regex_replace`                  | A unique identifier for the operator. |
| `output`       | Next in pipeline                 | The connected operator(s) that will receive all outbound entries. |
| `field`        | required                         | The [field](../types/field.md) to strip. Must be a string. |
| `regex`        | `regex` or `regex_name` required | A [Go regular expression](https://github.com/google/re2/wiki/Syntax). |
| `regex_name`   | `regex` or `regex_name` required | A well-known regex to use. See below for a list of possible values. |
| `replace_with` | optional                         | The [field](../types/field.md) to strip. Must be a string. |
| `on_error`     | `send`                           | The behavior of the operator if it encounters an error. See [on_error](../types/on_error.md). |
| `if`           |                                  | An [expression](../types/expression.md) that, when set, will be evaluated to determine whether this operator should be used for the given entry. This allows you to do easy conditional parsing without branching logic with routers. |

#### Well-known regular expressions

| Name                     | Description |
| ---                      | ---         |
| `ansi_control_sequences` | ANSI "Control Sequence Introducer (CSI)" escape codes starting with `ESC [` |

### Example Configurations

#### Collapse spaces 

Configuration:
```yaml
- type: regex_replace
  regex: " +"
  replace_with: " "
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
  "body": "Hello  World"
}
```

</td>
<td>

```json
{
  "resource": { },
  "attributes": { },
  "body": "Hello World"
}
```

</td>
</tr>
</table>

#### Match and replace with groups

Configuration:
```yaml
- type: regex_replace
  regex: "{(.*)}"
  replace_with: "${1}"
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
  "body": "{a}{bb}{ccc}"
}
```

</td>
<td>

```json
{
  "resource": { },
  "attributes": { },
  "body": "abbccc"
}
```

</td>
</tr>
</table>

#### Remove all ANSI color escape codes from the body

Configuration:
```yaml
- type: regex_replace
  regex_name: ansi_control_sequences
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
