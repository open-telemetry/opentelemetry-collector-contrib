## `Retain` operator

The `retain` operator keeps the specified list of fields, and removes the rest.

### Configuration Fields

| Field      | Default          | Description |
| ---        | ---              | ---         |
| `id`       | `retain`         | A unique identifier for the operator. |
| `output`   | Next in pipeline | The connected operator(s) that will receive all outbound entries. |
| `fields`   | required         | A list of [fields](../types/field.md) to be kept. |
| `on_error` | `send`           | The behavior of the operator if it encounters an error. See [on_error](../types/on_error.md). |
| `if`       |                  | An [expression](../types/expression.md) that, when set, will be evaluated to determine whether this operator should be used for the given entry. This allows you to do easy conditional parsing without branching logic with routers. |
<hr>
<b>NOTE:</b> If no fields in a group (attributes, resource, or body) are specified, that entire group will be retained.
<hr>

### Example Configurations:

<hr>
Retain fields in the body

```yaml
- type: retain
  fields:
    - body.key1
    - body.key2
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
    "key2": "val2",
    "key3": "val3",
    "key4": "val4"
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
Retain an object in the body

```yaml
- type: retain
  fields:
    - body.object
```

<table>
<tr><td> Input record </td> <td> Output record </td></tr>
<tr>
<td>

```json
{
  "resource": { },
  "attributes": { },
  "body": {
    "key1": "val1",
    "object": {
      "nestedkey": "val2",
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
    "object": {
      "nestedkey": "val2",
    }
  }
}
```

</td>
</tr>
</table>

<hr>
Retain fields from resource

```yaml
- type: retain
  fields:
    - resource.key1
    - resource.key2
```

<table>
<tr><td> Input record </td> <td> Output record </td></tr>
<tr>
<td>

```json
{
  "resource": {
     "key1": "val1",
     "key2": "val2",
     "key3": "val3"
  },
  "attributes": { },
  "body": {
    "key1": "val1",
    }
  }
}
```

</td>
<td>

```json
{
  "resource": {
     "key1": "val1",
     "key2": "val2",
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

<hr>
Retain fields from attributes

```yaml
- type: retain
  fields:
    - attributes.key1
    - attributes.key2
```

<table>
<tr><td> Input record </td> <td> Output record </td></tr>
<tr>
<td>

```json
{
  "resource": { },
  "attributes": {
     "key1": "val1",
     "key2": "val2",
     "key3": "val3"
  },
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
     "key1": "val1",
     "key2": "val2",
  },
  "body": {
    "key1": "val1",
  }
}
```

</td>
</tr>
</table>

<hr>
Retain fields from all sources

```yaml
- type: retain
  fields:
    - resource.key1
    - attributes.key3
    - body.key5
```

<table>
<tr><td> Input record </td> <td> Output record </td></tr>
<tr>
<td>

```json
{
  "resource": {
     "key1": "val1",
     "key2": "val2"
  },
  "attributes": {
     "key3": "val3",
     "key4": "val4"
  },
  "body": {
    "key5": "val5",
    "key6": "val6",
  }
}
```

</td>
<td>

```json
{
  "resource": {
     "key1": "val1",
  },
  "attributes": {
     "key3": "val3",
  },
  "body": {
    "key5": "val5",
  }
}
```

</td>
</tr>
</table>