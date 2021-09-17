## `restructure` operator

The `restructure` operator facilitates changing the structure of a body by adding, removing, moving, and flattening fields.

The operator is configured with a list of ops, which are small operations that are applied to a body in the order
they are defined.

### Configuration Fields

| Field      | Default          | Description |
| ---        | ---              | ---         |
| `id`       | `restructure`    | A unique identifier for the operator. |
| `output`   | Next in pipeline | The connected operator(s) that will receive all outbound entries. |
| `ops`      | required         | A list of ops. The available op types are defined below. |
| `on_error` | `send`           | The behavior of the operator if it encounters an error. See [on_error](/docs/types/on_error.md). |
| `if`       |                  | An [expression](/docs/types/expression.md) that, when set, will be evaluated to determine whether this operator should be used for the given entry. This allows you to do easy conditional parsing without branching logic with routers. |

### Op types

#### Add

The `add` op adds a field to a body. It must have a `field` key and exactly one of `value` or `value_expr`.

`field` is a [field](/docs/types/field.md) that will be set to `value` or the result of `value_expr`

`value` is a static string that will be added to each entry at the field defined by `field`

`value_expr` is an [expression](/docs/types/expression.md) with access to the `body` object

Example usage:
```yaml
- type: restructure
  ops:
    - add:
        field: "key1"
        value: "val1"
    - add:
        field: "key2"
        value_expr: 'body["key1"] + "-suffix"'
```

<table>
<tr><td> Input body </td> <td> Output body </td></tr>
<tr>
<td>

```json
{}
```

</td>
<td>

```json
{
  "key1": "val1",
  "key2": "val1-suffix"
}
```

</td>
</tr>
</table>

#### Remove

The `remove` op removes a field from a body.

Example usage:
```yaml
- type: restructure
  ops:
    - remove: "key1"
```

<table>
<tr><td> Input body </td> <td> Output body </td></tr>
<tr>
<td>

```json
{
  "key1": "val1",
  "key2": "val2"
}
```

</td>
<td>

```json
{
  "key2": "val2"
}
```

</td>
</tr>
</table>

#### Retain

The `retain` op keeps the specified list of fields, and removes the rest.

Example usage:
```yaml
- type: restructure
  ops:
    - retain:
      - "key1"
      - "key2"
```

<table>
<tr><td> Input body </td> <td> Output body </td></tr>
<tr>
<td>

```json
{
  "key1": "val1",
  "key2": "val2",
  "key3": "val3",
  "key4": "val4"
}
```

</td>
<td>

```json
{
  "key1": "val1",
  "key2": "val2"
}
```

</td>
</tr>
</table>

#### Move

The `move` op moves (or renames) a field from one location to another. Both the `from` and `to` fields are required.

Example usage:
```yaml
- type: restructure
  ops:
    - move:
        from: "key1"
        to: "key3"
```

<table>
<tr><td> Input body </td> <td> Output body </td></tr>
<tr>
<td>

```json
{
  "key1": "val1",
  "key2": "val2"
}
```

</td>
<td>

```json
{
  "key3": "val1",
  "key2": "val2"
}
```

</td>
</tr>
</table>

#### Flatten

The `flatten` op flattens a field by moving its children up to the same level as the field.

Example usage:
```yaml
- type: restructure
  ops:
    - flatten: "key1"
```

<table>
<tr><td> Input body </td> <td> Output body </td></tr>
<tr>
<td>

```json
{
  "key1": {
    "nested1": "nestedval1",
    "nested2": "nestedval2"
  },
  "key2": "val2"
}
```

</td>
<td>

```json
{
  "nested1": "nestedval1",
  "nested2": "nestedval2",
  "key2": "val2"
}
```

</td>
</tr>
</table>
