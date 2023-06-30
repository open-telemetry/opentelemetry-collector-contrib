# OpenSearch Exporter

| Status                   |               |
| ------------------------ |---------------|
| Stability                | [development] |
| Supported pipeline types | traces        |
| Distributions            | [contrib]     |

This exporter supports sending OpenTelemetry signals as documents to [OpenSearch](https://www.opensearch.org).

The documents are sent using [observability catalog](https://github.com/opensearch-project/opensearch-catalog/tree/main/schema/observability) schema.

## Getting Started

The settings are:

- `endpoint` (required, default = 0.0.0.0:3500 for grpc protocol, 0.0.0.0:3600 http protocol): host:port to which the receiver is going to receive data.
- `use_incoming_timestamp` (optional, default = false) if set `true` the timestamp from Loki log entry is used

Example:
```yaml
exporters:
  opensearch/trace:
    endpoints: [https://opensearch.example.com:9200]
# ······
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [opensearch/trace]
      processors: [batch]
```

[development]:https://github.com/open-telemetry/opentelemetry-collector#development
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib