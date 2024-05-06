## Filtering log messages based on content

Filelog receiver provides support for filtering logs based on their content. This can be achieved by using
the [filter operator](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/stanza/docs/operators/filter.md),
configured with matching regular expressions.

With this happening at the collection point, a lot of resources at the destination backend
can be saved since no additional processing would need to take place.

A full configuration example on how to filter out logs that start with the `INFO:` pattern is
provided in the [example config](./otel-col-config-filter-out-logs.yaml).
A full configuration example on how to only collect logs that start with the `WARN:` pattern is provided in
the [example config](./otel-col-config-filter-in-logs.yaml)