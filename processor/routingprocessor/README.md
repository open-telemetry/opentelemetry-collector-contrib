# Routing processor

Routes logs, metrics or traces to specific exporters.

This processor will either read a header from the incoming HTTP request (gRPC or plain HTTP), or it will read a resource attribute, and direct the trace information to specific exporters based on the value read.

This processor *does not* let traces to continue through the pipeline and will emit a warning in case other processor(s) are defined after this one.
Similarly, exporters defined as part of the pipeline are not authoritative: if you add an exporter to the pipeline, make sure you add it to this processor *as well*, otherwise it won't be used at all.
All exporters defined as part of this processor *must also* be defined as part of the pipeline's exporters.

Given that this processor depends on information provided by the client via HTTP headers or resource attributes, caution must be taken when processors that aggregate data like `batch` or `groupbytrace` are used as part of the pipeline.

The following settings are required:

- `from_attribute`: contains the HTTP header name or the resource attribute name to look up the route's value. Only the OTLP exporter has been tested in connection with the OTLP gRPC Receiver, but any other gRPC receiver should work fine, as long as the client sends the specified HTTP header.
- `table`: the routing table for this processor.
- `table.value`: a possible value for the attribute specified under FromAttribute.
- `table.exporters`: the list of exporters to use when the value from the FromAttribute field matches this table item.

The following settings can be optionally configured:

- `attribute_source` defines where to look for the attribute in `from_attribute`. The allowed values are:
  - `context` (the default) - to search the [context][context_docs], which includes HTTP headers
  - `resource` - to search the resource attributes.
- `drop_resource_routing_attribute` - controls whether to remove the resource attribute used for routing. This is only relevant if AttributeSource is set to resource.
- `default_exporters` contains the list of exporters to use when a more specific record can't be found in the routing table.

Example:

```yaml
processors:
  routing:
    from_attribute: X-Tenant
    default_exporters:
    - jaeger
    table:
    - value: acme
      exporters: [jaeger/acme]
exporters:
  jaeger:
    endpoint: localhost:14250
  jaeger/acme:
    endpoint: localhost:24250
```

The full list of settings exposed for this processor are documented [here](./config.go) with detailed sample configuration files:

- [logs](./testdata/config_logs.yaml)
- [metrics](./testdata/config_metrics.yaml)
- [traces](./testdata/config_traces.yaml)

[context_docs]: https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/context/context.md
