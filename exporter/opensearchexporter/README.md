# OpenSearch Exporter

| Status                   |           |
| ------------------------ |-----------|
| Stability                | [devel]   |
| Supported pipeline types | traces    |
| Distributions            | [contrib] |

OpenSearch exporter supports sending OpenTelemetry signals as documents to [OpenSearch](https://www.opensearch.org).

The documents are sent using [observability catalog](https://github.com/opensearch-project/opensearch-catalog/tree/main/schema/observability) schema.

## Configuration options
### Indexing Options
- `dataset` (default=`default`) a user-provided label to classify source of telemetry. It is used to construct the name of the destination index or data stream.
- `namespace` (default=`namespace`) a user-provided label to group telemetry. It is used to construct the name of the destination index or data stream.

### HTTP Connection Options
OpenSearch export supports standard (HTTP client settings](https://github.com/open-telemetry/opentelemetry-collector/tree/main/config/confighttp#client-configuration).
- `endpoint` (required) `<url>:<port>` of OpenSearch node to send data to.

### TLS settings
Supports standard TLS settings as part of HTTP settings. See [TLS Configuration/Client Settings](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/configtls/README.md#client-configuration).

### Retry Options
- `retry_on_failure`: See [retry_on_failure](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/exporterhelper/README.md)

## Example

```yaml
extensions:
  basicauth/client:
  client_auth:
    username: username
    password: password
    
exporters:
  opensearch/trace:
    endpoint: https://opensearch.example.com:9200
    auth:
      authenticator: basicauth/client
# ······
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [opensearch/trace]
      processors: [batch]
```
[devel]:https://github.com/open-telemetry/opentelemetry-collector#development
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib