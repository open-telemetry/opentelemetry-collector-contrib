## `noop` operator

The `noop` operator makes no changes to a entry. It is sometimes useful as a terminal node in [non-linear pipelines](../types/operators.md#non-linear-sequences).

### Configuration Fields

| Field      | Default          | Description |
| ---        | ---              | ---         |
| `id`       | `noop`           | A unique identifier for the operator. |
| `output`   | Next in pipeline | The connected operator(s) that will receive all outbound entries. |


### Example Configuration:

Process logs according to some criteria, then direct all logs to a `noop` operator before emitting from the receiver.

```yaml
operators:
  - type: router
    routes:
      - output: json_parser
        expr: 'body.format == "json"'
      - output: syslog_parser
        expr: 'body.format == "syslog"'
  - type: json_parser
    output: noop  # If this were not set, logs would implicitly pass to the next operator
  - type: syslog_parser
  - type: noop
```

#### Why is this necessary?

The last operator is always responsible for emitting logs from the receiver. In non-linear pipelines, it is sometimes necessary to explictly direct logs to the final operator. In many such cases, the final operator performs some work. However, if no more work is required, the `noop` operator can serve as a final operator.
