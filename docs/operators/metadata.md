## `metadata` operator

The `metadata` operator adds labels to incoming entries.

### Configuration Fields

| Field      | Default          | Description                                                                                     |
| ---        | ---              | ---                                                                                             |
| `id`       | `metadata`       | A unique identifier for the operator                                                            |
| `output`   | Next in pipeline | The connected operator(s) that will receive all outbound entries                                |
| `labels`   | {}               | A map of `key: value` labels to add to the entry's labels                                       |
| `resource` | {}               | A map of `key: value` labels to add to the entry's resource                                     |
| `on_error` | `send`           | The behavior of the operator if it encounters an error. See [on_error](/docs/types/on_error.md) |

Inside the label values, an [expression](/docs/types/expression.md) surrounded by `EXPR()`
will be replaced with the evaluated form of the expression. The entry's record can be accessed
with the `$` variable in the expression so labels can be added dynamically from fields.

### Example Configurations


#### Add static labels and resource

Configuration:
```yaml
- type: metadata
  labels:
    environment: "production"
  resource:
    cluster: "blue"
```

<table>
<tr><td> Input entry </td> <td> Output entry </td></tr>
<tr>
<td>

```json
{
  "timestamp": "2020-06-15T11:15:50.475364-04:00",
  "labels": {},
  "record": {
    "message": "test"
  }
}
```

</td>
<td>

```json
{
  "timestamp": "2020-06-15T11:15:50.475364-04:00",
  "labels": {
    "environment": "production"
  },
  "resource": {
    "cluster": "blue"
  },
  "record": {
    "message": "test"
  }
}
```

</td>
</tr>
</table>

#### Add dynamic tags and labels

Configuration:
```yaml
- type: metadata
  output: metadata_receiver
  labels:
    environment: 'EXPR( $.environment == "production" ? "prod" : "dev" )'
```

<table>
<tr><td> Input entry </td> <td> Output entry </td></tr>
<tr>
<td>

```json
{
  "timestamp": "2020-06-15T11:15:50.475364-04:00",
  "labels": {},
  "record": {
    "production_location": "us_east",
    "environment": "nonproduction"
  }
}
```

</td>
<td>

```json
{
  "timestamp": "2020-06-15T11:15:50.475364-04:00",
  "labels": {
    "environment": "dev"
  },
  "record": {
    "production_location": "us_east",
    "environment": "nonproduction"
  }
}
```

</td>
</tr>
</table>
