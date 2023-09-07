## `move` operator

The `move` operator moves (or renames) a field from one location to another.

### Configuration Fields

| Field      | Default          | Description |
| ---        | ---              | ---         |
| `id`       | `move`           | A unique identifier for the operator. |
| `output`   | Next in pipeline | The connected operator(s) that will receive all outbound entries. |
| `from`     | required         | The [field](../types/field.md) from which the value will be moved. |
| `to`       | required         | The [field](../types/field.md) to which the value will be moved. |
| `on_error` | `send`           | The behavior of the operator if it encounters an error. See [on_error](../types/on_error.md). |
| `if`       |                  | An [expression](../types/expression.md) that, when set, will be evaluated to determine whether this operator should be used for the given entry. This allows you to do easy conditional parsing without branching logic with routers. |

### Example Configurations:

Rename value
```yaml
- type: move
  from: body.key1
  to: body.key3
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
  "attributes": { },
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

Move a value from the body to resource

```yaml
- type: move
  from: body.uuid
  to: resource.uuid
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
    "uuid": "091edc50-d91a-460d-83cd-089a62937738"
  }
}
```

</td>
<td>

```json
{
  "resource": {
    "uuid": "091edc50-d91a-460d-83cd-089a62937738"
  },
  "attributes": { },
  "body": { }
}
```

</td>
</tr>
</table>

<hr>

Move a value from the body to attributes

```yaml
- type: move
  from: body.ip
  to: attributes.ip
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
    "ip": "8.8.8.8"
  }
}
```

</td>
<td>

```json
{
  "resource": { },
  "attributes": {
    "ip": "8.8.8.8"
  },
  "body": { }
}
```

</td>
</tr>
</table>

<hr>

Replace the body with an individual value nested within the body
```yaml
- type: move
  from: body.log
  to: body
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
    "log": "The log line"
  }
}
```

</td>
<td>

```json
{
  "resource": { },
  "attributes": { },
  "body": "The log line"
}
```

</td>
</tr>
</table>

<hr>

Remove a layer from the body
```yaml
- type: move
  from: body.wrapper
  to: body
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
    "wrapper": {
      "key1": "val1",
      "key2": "val2",
      "key3": "val3"
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
    "key1": "val1",
    "key2": "val2",
    "key3": "val3"
  }
}
```

</td>
</tr>
</table>

<hr>

Merge a layer to the body
```yaml
- type: move
  from: body.object
  to: body
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
    "firstTimestamp": "2020-08-13T16:43:57Z",
    "object": {
      "apiVersion": "v1",
      "name": "stanza-g6rzd",
      "uid": "47d965e6-4bb3-4c58-a089-1a8b16bf21b0"
    },
    "lastTimestamp": "2020-08-13T16:43:57Z",
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
    "firstTimestamp": "2020-08-13T16:43:57Z",
    "apiVersion": "v1",
    "name": "stanza-g6rzd",
    "uid": "47d965e6-4bb3-4c58-a089-1a8b16bf21b0",
    "lastTimestamp": "2020-08-13T16:43:57Z",
  }
}
```

</td>
</tr>
</table>

