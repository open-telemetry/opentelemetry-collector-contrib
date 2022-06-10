## `filter` operator

The `filter` operator filters incoming entries that match an expression.

### Configuration Fields

| Field        | Default          | Description |
| ---          | ---              | ---         |
| `id`         | `filter`         | A unique identifier for the operator. |
| `output`     | Next in pipeline | The connected operator(s) that will receive all outbound entries. |
| `expr`       | required         | Incoming entries that match this [expression](../types/expression.md) will be dropped. |
| `drop_ratio` | 1.0              | The probability a matching entry is dropped (used for sampling). A value of 1.0 will drop 100% of matching entries, while a value of 0.0 will drop 0%. |

### Examples

#### Filter entries based on a regex pattern

```yaml
- type: filter
  expr: 'body.message matches "^LOG: .* END$"'
  output: my_output
```

#### Filter entries based on a label value

```yaml
- type: filter
  expr: 'attributes.env == "production"'
  output: my_output
```

#### Filter entries based on an environment variable

```yaml
- type: filter
  expr: 'body.message == env("MY_ENV_VARIABLE")'
  output: my_output
```
