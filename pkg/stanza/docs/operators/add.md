## `add` operator

The `add` operator adds a value to an `entry`'s `body`, `attributes`, or `resource`.

### Configuration Fields

| Field      | Default          | Description |
| ---        | ---              | ---         |
| `id`       | `add`            | A unique identifier for the operator. |
| `output`   | Next in pipeline | The connected operator(s) that will receive all outbound entries. |
| `field`    | required         | The [field](../types/field.md) to be added. |
| `value`    | required         | `value` is either a static value or an [expression](../types/expression.md). If a value is specified, it will be added to each entry at the field defined by `field`. If an expression is specified, it will be evaluated for each entry and added at the field defined by `field`. |
| `on_error` | `send`           | The behavior of the operator if it encounters an error. See [on_error](../types/on_error.md). |
| `if`       |                  | An [expression](../types/expression.md) that, when set, will be evaluated to determine whether this operator should be used for the given entry. This allows you to do easy conditional parsing without branching logic with routers. |


### Example Configurations:

<hr>
Add a string to the body

```yaml
- type: add
  field: body.key2
  value: val2
```

<table>
<tr><td> Input entry </td> <td> Output entry </td></tr>
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
  "body": {
    "key1": "val1",
    "key2": "val2"
  }
}
```

</td>
</tr>
</table>

<hr>
Add a value to the body using an expression

```yaml
- type: add
  field: body.key2
  value: EXPR(body.key1 + "_suffix")
```

<table>
<tr><td> Input entry </td> <td> Output entry </td></tr>
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
  "body": {
    "key1": "val1",
    "key2": "val1_suffix"
  }
}
```

</td>
</tr>
</table>

<hr>
Add an object to the body

```yaml
- type: add
  field: body.key2
  value:
    nestedkey: nestedvalue
```

<table>
<tr><td> Input entry </td> <td> Output entry </td></tr>
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
  "body": {
    "key1": "val1",
    "key2": {
      "nestedkey":"nestedvalue"
    }
  }
}
```

</td>
</tr>
</table>

<hr>
Add a value to attributes

```yaml
- type: add
  field: attributes.key2
  value: val2
```

<table>
<tr><td> Input entry </td> <td> Output entry </td></tr>
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
  "attributes": {
     "key2": "val2"
  },
  "body": {
    "key1": "val1"
  }
}
```

</td>
</tr>
</table>

<hr>
Add a value to resource

```yaml
- type: add
  field: resource.key2
  value: val2
```

<table>
<tr><td> Input entry </td> <td> Output entry </td></tr>
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
  "resource": {
    "key2": "val2"
  },
  "attributes": { },
  "body": {
    "key1": "val1"
  }
}
```

</td>
</tr>
</table>

Add a value to resource using an expression

```yaml
- type: add
  field: resource.key2
  value: EXPR(body.key1 + "_suffix")
```

<table>
<tr><td> Input entry </td> <td> Output entry </td></tr>
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
  "resource": {
    "key2": "val1_suffix"
  },
  "attributes": { },
  "body": {
    "key1": "val1",
  }
}
```

</td>
</tr>
</table>
