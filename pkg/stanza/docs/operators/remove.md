## `remove` operator

The `remove` operator removes a field from a record.

### Configuration Fields

| Field      | Default          | Description |
| ---        | ---              | ---         |
| `id`       | `remove`         | A unique identifier for the operator. |
| `output`   | Next in pipeline | The connected operator(s) that will receive all outbound entries. |
| `field`    | required         | The [field](../types/field.md) to remove. if 'attributes' or 'resource' is specified, all fields of that type will be removed. |
| `on_error` | `send`           | The behavior of the operator if it encounters an error. See [on_error](../types/on_error.md). |
| `if`       |                  | An [expression](../types/expression.md) that, when set, will be evaluated to determine whether this operator should be used for the given entry. This allows you to do easy conditional parsing without branching logic with routers. |

### Example Configurations:

<hr>

Remove a value from the body
```yaml
- type: remove
  field: body.key1
```

<table>
<tr><td> Input Entry </td> <td> Output Entry </td></tr>
<tr>
<td>

```json
{
  "resource": { },
  "attributes": { },
  "body": {
    "key1": "val1",
  }
}
```

</td>
<td>

```json
{
  "resource": { },
  "attributes": { },
  "body": { }
}
```

</td>
</tr>
</table>

<hr>

Remove an object from the body
```yaml
- type: remove
  field: body.object
```

<table>
<tr><td> Input Entry </td> <td> Output Entry </td></tr>
<tr>
<td>

```json
{
  "resource": { },
  "attributes": { },
  "body": {
    "object": {
      "nestedkey": "nestedval"
    },
    "key": "val"
  },
}
```

</td>
<td>

```json
{
  "resource": { },
  "attributes": { },
  "body": {
     "key": "val"
  }
}
```

</td>
</tr>
</table>

<hr>

Remove a value from attributes
```yaml
- type: remove
  field: attributes.otherkey
```

<table>
<tr><td> Input Entry </td> <td> Output Entry </td></tr>
<tr>
<td>

```json
{
  "resource": { },
  "attributes": {
    "otherkey": "val"
  },
  "body": {
    "key": "val"
  },
}
```

</td>
<td>

```json
{
  "resource": { },
  "attributes": {  },
  "body": {
    "key": "val"
  }
}
```

</td>
</tr>
</table>

<hr>

Remove a value from resource
```yaml
- type: remove
  field: resource.otherkey
```

<table>
<tr><td> Input Entry </td> <td> Output Entry </td></tr>
<tr>
<td>

```json
{
  "resource": {
    "otherkey": "val"
  },
  "attributes": {  },
  "body": {
    "key": "val"
  },
}
```

</td>
<td>

```json
{
  "resource": { },
  "attributes": { },
  "body": {
    "key": "val"
  }
}
```

</td>
</tr>
</table>

<hr>

Remove all resource fields
```yaml
- type: remove
  field: resource
```

<table>
<tr><td> Input Entry </td> <td> Output Entry </td></tr>
<tr>
<td>

```json
{
  "resource": {
    "key1.0": "val",
    "key2.0": "val"
  },
  "attributes": {  },
  "body": {
    "key": "val"
  },
}
```

</td>
<td>

```json
{
  "resource": { },
  "attributes": { },
  "body": {
    "key": "val"
  }
}
```

</td>
</tr>
</table>

<hr>

Remove all attributes
```yaml
- type: remove
  field: attributes
```

<table>
<tr><td> Input Entry </td> <td> Output Entry </td></tr>
<tr>
<td>

```json
{
  "resource": {  },
  "attributes": {
    "key1.0": "val",
    "key2.0": "val"
  },
  "body": {
    "key": "val"
  },
}
```

</td>
<td>

```json
{
  "resource": { },
  "attributes": { },
  "body": {
    "key": "val"
  }
}
```

</td>
</tr>
</table>