## `assign_keys` operator

The `assign_keys` assigns keys from the configuration to an input list. the output is a map containing these key-value pairs

### Configuration Fields

| Field      | Default          | Description |
| ---        | ---              | ---         |
| `id`       | `assign_keys`        | A unique identifier for the operator. |
| `output`   | Next in pipeline | The connected operator(s) that will receive all outbound entries. |
| `field`    | required         | The [field](../types/field.md) to assign keys to. |
| `keys`    | required         | The list of strings to be used as the keys to the input list's values. Its length is expected to be equal to the length of the values list from field. In case there is a mismatch, an error will result. |
| `on_error` | `send`           | The behavior of the operator if it encounters an error. See [on_error](../types/on_error.md). |
| `if`       |                  | An [expression](../types/expression.md) that, when set, will be evaluated to determine whether this operator should be used for the given entry. This allows you to do easy conditional parsing without branching logic with routers. |

### Example Configurations:

<hr>
Assign keys to a list in body
<br>
<br>

```yaml
- type: assign_keys
  field: body
  keys: ["foo", "bar", "charlie", "foxtrot"]
```

<table>
<tr><td> Input Entry </td> <td> Output Entry </td></tr>
<tr>
<td>

```json
{
  "body": [1, "debug", "Debug Message", true]
}
```

</td>
<td>

```json
  {
    "body": {
      "foo": 1,
      "bar": "debug",
      "charlie": "Debug Message",
      "foxtrot": true,
    }
  }
```

</td>
</tr>
</table>
<hr>
Assign keys to a list in an attributes field
<br>
<br>

```yaml
- type: assign_keys
  field: attributes.input
  keys: ["foo", "bar"]
```

<table>
<tr><td> Input Entry </td> <td> Output Entry </td></tr>
<tr>
<td>

```json
{
  "attributes": {
    "input": [1, "debug"],
    "field2": "unchanged",
  }
}
```

</td>
<td>

```json
{
  "attributes": {
    "input": {
      "foo": 1, 
      "bar": "debug",
    },
    "field2": "unchanged",
  }
}
```

</td>
</tr>
</table>