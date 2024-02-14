# Operator Sequences

An operator sequence is made up of [operators](../operators/README.md) and defines how logs should be parsed and filtered before being emitted from a receiver.

## Linear Sequences

In a linear sequence of operators, logs flow from one operator to the next according to the order in which they are defined.

For example, the following sequence reads logs from a file, parses them as `json`, removes a particular attribute, and adds another.

```yaml
receivers:
  filelog:
    include: my-log.json
    operators:
      - type: json_parser
      - type: remove
        field: attributes.foo
      - type: add
        key: attributes.bar
        value: baz
```

Notice that every operator has a `type` field. The `type` of operator must always be specified.

## `id` and `output`

Linear sequences are sufficient for many use cases, but it is also possible to define non-linear sequences as well. In order to use non-linear sequences, the `id` and `output` fields must be understood. Let's take a close look at these.

Each operator has a unique `id`. By default, `id` will take the same value as `type`. Alternately, you can specify a custom `id` for any operator.

All operators support an `output` field which refers to the `id` of another operator. By default, the output field takes the value of the next operator's `id`.

_The final operator in a sequence automatically emits logs from the receiver._

Let's look at how these default values work together by considering the linear sequence shown above. The following pipeline would be exactly the same (although more verbosely defined):

```yaml
receivers:
  filelog:
    include: my-log.json
    operators:
      - type: json_parser
        id: json_parser
        output: remove
      - type: remove
        id: remove
        field: attributes.foo
        output: add
      - type: add
        id: add
        key: attributes.bar
        value: baz
        # the last operator automatically outputs from the receiver
```

Additionally, we could accomplish the same task using custom `id`'s.

```yaml
receivers:
  filelog:
    include: my-log.json
    operators:
      - type: json_parser
        id: my_json_parser
        output: my_remove
      - type: remove
        id: my_remove
        field: attributes.foo
        output: my_add
      - type: add
        id: my_add
        key: attributes.bar
        value: baz
        # the last operator automatically outputs from the receiver
```

## Non-Linear Sequences

Now that we understand how `id` and `output` work together, we can configure more complex sequences. Technically, we are only limited in that the relationship between operators must be a [directed, acyclic, graph](https://en.wikipedia.org/wiki/Directed_acyclic_graph).

Here's a scenario where we read from a file that contains logs with two differnet formats which must be parsed differently:

```yaml
receivers:
  filelog:
    include: my-log.json
    operators:
      - type: router
        routes:
          - expr: 'body matches "^{.*}$"'
            output: json_parser
          - expr: 'body startsWith "ERROR"'
            output: error_parser
      - type: json_parser
        output: remove # send from here directly to the 'remove' operator
      - type: regex_parser
        id: error_parser
        regex: ... # regex appropriate to parsing error logs
      - type: remove
        field: attributes.foo
      - type: add
        key: attributes.bar
        value: baz
```

### Emitting from a reciever

By default, the last operator in a sequence will emit logs from the receiver.

However, in some non-linear sequences, you may not want all logs to flow through the last operator. In such cases, you can add a `noop` operator to the end of the sequence which will have no effect on the logs, but _will_ emit them from the receiver.

```yaml
receivers:
  filelog:
    include: my-log.json
    operators:
      - type: router
        routes:
          - expr: 'body matches "^{.*}$"'
            output: json_parser
          - expr: 'body startsWith "ERROR"'
            output: error_parser
      - type: json_parser
        output: noop # send from here directly to the end of the sequence
      - type: regex_parser
        id: error_parser
        regex: ... # regex appropriate to parsing error logs
      - type: noop
```
