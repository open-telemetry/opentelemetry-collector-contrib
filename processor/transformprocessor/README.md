# Transform Processor

Supported pipeline types: logs, traces

The transform processor modifies telemetry based on configuration using the Telemetry Query Language.
It takes a list of queries which are performed in the order specified in the config.

Queries are composed of the following parts
- Path expressions: Fields within the incoming data can be referenced using expressions composed of the names as defined
in the OTLP protobuf definition. e.g., `status.code`, `attributes["http.method"]`. If the path expression begins with
`resource.` or `instrumentation_library.`, it will reference those values.
  - The name `instrumentation_library` within OpenTelemetry is currently under discussion and may be changed in the future.
- Literals: Strings, ints, and floats can be referenced as literal values
- Function invocations: Functions can be invoked with arguments matching the function's expected arguments
- Where clause: Telemetry to modify can be filtered by appending `where a <op> b`, with `a` and `b` being any of the above.

Supported functions:
- `set(target, value)` - `target` is a path expression to a telemetry field to set `value` into. `value` is any value type.
e.g., `set(attributes["http.path"], "/foo")`, `set(name, attributes["http.route"])`. If `value` resolves to `nil`, e.g.
it references an unset map value, there will be no action.

- `keep_keys(target, string...)` - `target` is a path expression to a map type field. The map will be mutated to only contain
the fields specified by the list of strings. e.g., `keep_keys(attributes, "http.method")`, `keep_keys(attributes, "http.method", "http.route")`

- `limit(target, limit)` - `target` is a path expression to a map type field. `limit` is an integer.  The map will be mutated such that the number of items does not exceed the limit. e.g., `truncate(attributes, 100)` will limit `attributes` to no more than 100 items. Which items are dropped is random.

Supported where operations:
- `==` - matches telemetry where the values are equal to each other
- `!=` - matches telemetry where the values are not equal to each other

Example configuration:
```yaml
receivers:
  otlp:
    protocols:
      grpc:

exporters:
  nop

processors:
  transform:
    logs:
      queries:
        - set(severity_text, "FAIL") where body == "request failed"
        - keep_keys(resource.attributes, "service.name", "service.namespace", "cloud.region")
        - set(body, attributes["http.route"])
    traces:
      queries:
        - set(status.code, 1) where attributes["http.path"] == "/health"
        - keep_keys(resource.attributes, "service.name", "service.namespace", "cloud.region")
        - set(name, attributes["http.route"])
        - limit(attributes, 100)
        - limit(resource.attributes, 100)
service:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [transform]
      exporters: [nop]
    traces:
      receivers: [otlp]
      processors: [transform]
      exporters: [nop]
```

This processor will perform the operations in order for 

All logs

1) Set severity text to FAIL if the body contains a string text "request failed"
2) Keep only `service.name`, `service.namespace`, `cloud.region` resource attributes
3) Set `body` to the `http.route` attribute if it is set

All spans

1) Set status code to OK for all spans with a path `/health`
2) Keep only `service.name`, `service.namespace`, `cloud.region` resource attributes
3) Set `name` to the `http.route` attribute if it is set
4) Limit all span attributes such that each span has no more than 100 attributes.
5) Limit all resource attributes such that each resource no more than 100 attributes.
