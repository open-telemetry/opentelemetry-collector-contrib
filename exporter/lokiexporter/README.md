# Loki Exporter

Exports data via HTTP to [Loki](https://grafana.com/docs/loki/latest/).

Supported pipeline types: logs

## Getting Started

The following settings are required:

- `endpoint` (no default): The target URL to send Loki log streams to (
  e.g.: https://loki.example.com:3100/loki/api/v1/push).


- `attributes_for_labels` (no default): List of allowed attributes to be added as labels to Loki log 
  streams. This is a safety net to help prevent accidentally adding dynamic labels that may significantly increase 
  cardinality, thus having a performance impact on your Loki instance. See the 
  [Loki label best practices](https://grafana.com/docs/loki/latest/best-practices/current-best-practices/) page for 
  additional details on the types of labels you may want to associate with log streams.

The following settings can be optionally configured:

- `insecure` (default = false): When set to true disables verifying the server's certificate chain and host name. The
  connection is still encrypted but server identity is not verified.
- `ca_file` (no default) Path to the CA cert to verify the server being connected to. Should only be used if `insecure` 
  is set to false.
- `cert_file` (no default) Path to the TLS cert to use for client connections when TLS client auth is required. 
  Should only be used if `insecure` is set to false.
- `key_file` (no default) Path to the TLS key to use for TLS required connections. Should only be used if `insecure` is
  set to false.


- `timeout` (default = 30s): HTTP request time limit. For details see https://golang.org/pkg/net/http/#Client
- `read_buffer_size` (default = 0): ReadBufferSize for HTTP client.
- `write_buffer_size` (default = 512 * 1024): WriteBufferSize for HTTP client.


- `headers` (no default): Name/value pairs added to the HTTP request headers. Loki by default uses the "X-Scope-OrgID"
  header to identify the tenant the log is associated to.

Example:

```yaml
loki:
  endpoint: https://loki.example.com:3100/loki/api/v1/push
  attributes_for_labels:
    - container.name
    - k8s.cluster.name
    - severity
  headers:
    "X-Scope-OrgID": "example"
```

The full list of settings exposed for this exporter are documented [here](./config.go) with detailed sample
configurations [here](./testdata/config.yaml).

## Advanced Configuration

Several helper files are leveraged to provide additional capabilities automatically:

- [HTTP settings](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/confighttp/README.md)
- [Queuing and retry settings](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/exporterhelper/README.md)