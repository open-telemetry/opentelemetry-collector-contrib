## Scope Name Parsing

A Scope Name may be parsed from a log entry in order to indicate the code from which a log was emitted.

### `scope_name` parsing parameters

Parser operators can parse a scope name and attach the resulting value to a log entry.

| Field          | Default   | Description |
| ---            | ---       | ---         |
| `parse_from`   | required  | The [field](../types/field.md) from which the value will be parsed. |


### How to use `scope_name` parsing

All parser operators, such as [`regex_parser`](../operators/regex_parser.md) support these fields inside of a `scope_name` block.

If a `scope_name` block is specified, the parser operator will perform the parsing _after_ performing its other parsing actions, but _before_ passing the entry to the specified output operator.


### Example Configurations

#### Parse a scope_name from a string

Configuration:
```yaml
- type: regex_parser
  regexp: '^(?P<scope_name_field>\S*)\s-\s(?P<message>.*)'
  scope_name:
    parse_from: body.scope_name_field
```

<table>
<tr><td> Input entry </td> <td> Output entry </td></tr>
<tr>
<td>

```json
{
  "resource": { },
  "attributes": { },
  "body": "com.example.Foo - some message",
  "scope_name": "",
}
```

</td>
<td>

```json
{
  "resource": { },
  "attributes": { },
  "body": {
    "message": "some message",
  },
  "scope_name": "com.example.Foo",
}
```

</td>
</tr>
</table>
