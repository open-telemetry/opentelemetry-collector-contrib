## `copy` operator

The `copy` operator copies a value from one [field](../types/field.md) to another.

### Configuration Fields

| Field      | Default          | Description |
| ---        | ---              | ---         |
| `id`       | `copy`           | A unique identifier for the operator. |
| `output`   | Next in pipeline | The connected operator(s) that will receive all outbound entries. |
| `from`     | required         | The [field](../types/field.md) from which the value should be copied. |
| `to`       | required         | The [field](../types/field.md) to which the value should be copied. |
| `on_error` | `send`           | The behavior of the operator if it encounters an error. See [on_error](../types/on_error.md). |
| `if`       |                  | An [expression](../types/expression.md) that, when set, will be evaluated to determine whether this operator should be used for the given entry. This allows you to do easy conditional parsing without branching logic with routers. |

### Example Configurations:

<hr>
Copy a value from the body to resource

```yaml
- type: copy
  from: body.key
  to: resource.newkey
```

<table>
<tr><td> Input Entry</td> <td> Output Entry </td></tr>
<tr>
<td>

```json
{
  "resource": { },
  "attributes": { },
  "body": {
    "key":"value"
  }
}
```

</td>
<td>

```json
{
  "resource": {
       "newkey":"value"
  },
  "attributes": { },
  "body": {
    "key":"value"
  }
}
```

</td>
</tr>
</table>

<hr>

Copy a value from the body to attributes
```yaml
- type: copy
  from: body.key2
  to: attributes.newkey
```

<table>
<tr><td> Input Entry</td> <td> Output Entry </td></tr>
<tr>
<td>

```json
{
  "resource": { },
  "attributes": { },
  "body": {
    "key1": "val1",
    "key2": "val2"
  }
}
```

</td>
<td>

```json
{
  "resource": { },
  "attributes": {
      "newkey": "val2"
  },
  "body": {
    "key3": "val1",
    "key2": "val2"
  }
}
```

</td>
</tr>
</table>

<hr>

Copy a value from attributes to the body
```yaml
- type: copy
  from: attributes.key
  to: body.newkey
```

<table>
<tr><td> Input Entry</td> <td> Output Entry </td></tr>
<tr>
<td>

```json
{
  "resource": { },
  "attributes": {
      "key": "newval"
  },
  "body": {
    "key1": "val1",
    "key2": "val2"
  }
}
```

</td>
<td>

```json
{
  "resource": { },
  "attributes": {
      "key": "newval"
  },
  "body": {
    "key3": "val1",
    "key2": "val2",
    "newkey": "newval"
  }
}
```

</td>
</tr>
</table>

<hr>

Copy a value within the body
```yaml
- type: copy
  from: body.obj.nested
  to: body.newkey
```

<table>
<tr><td> Input Entry</td> <td> Output Entry </td></tr>
<tr>
<td>

```json
{
  "resource": { },
  "attributes": { },
  "body": {
      "obj": {
        "nested":"nestedvalue"
    }
  }
}
```

</td>
<td>

```json
{
  "resource": { },
  "attributes": { },
  "body": {
    "obj": {
        "nested":"nestedvalue"
    },
    "newkey":"nestedvalue"
  }
}
```

</td>
</tr>
</table>