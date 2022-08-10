## `router` operator

The `router` operator allows logs to be routed dynamically based on their content.

The operator is configured with a list of routes, where each route has an associated expression.
An entry sent to the router operator is forwarded to the first route in the list whose associated
expression returns `true`.

An entry that does not match any of the routes is dropped and not processed further.

### Configuration Fields

| Field     | Default  | Description |
| ---       | ---      | ---         |
| `id`      | `router` | A unique identifier for the operator. |
| `routes`  | required | A list of routes. See below for details. |
| `default` |          | The operator(s) that will receive any entries not matched by any of the routes. |

#### Route configuration

| Field        | Default  | Description |
| ---          | ---      | ---         |
| `output`     | required | The connected operator(s) that will receive all outbound entries for this route. |
| `expr`       | required | An [expression](../types/expression.md) that returns a boolean. The body of the routed entry is available as `$`. |
| `attributes` | {}       | A map of `key: value` pairs to add to an entry that matches the route. |


### Examples

#### Forward entries to different parsers based on content

```yaml
- type: router
  routes:
    - output: my_json_parser
      expr: 'body.format == "json"'
    - output: my_syslog_parser
      expr: 'body.format == "syslog"'
```

#### Drop entries based on content

```yaml
- type: router
  routes:
    - output: my_output
      expr: 'body.message matches "^LOG: .* END$"'
```

#### Route with a default

```yaml
- type: router
  routes:
    - output: my_json_parser
      expr: 'body.format == "json"'
  default: catchall
```
