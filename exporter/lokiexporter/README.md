# Loki Exporter

Exports data via HTTP to [Loki](https://grafana.com/docs/loki/latest/).

Supported pipeline types: logs

## Getting Started

The following settings are required:

- `endpoint` (no default): The target URL to send Loki log streams to (e.g.: http://loki:3100/loki/api/v1/push).
  
- `labels.{attributes/resource}` (no default): Either a map of attributes or resource names to valid Loki label names 
  (must match "^[a-zA-Z_][a-zA-Z0-9_]*$") allowed to be added as labels to Loki log streams. 
  Attributes are log record attributes that describe the log message itself. Resource attributes are attributes that 
  belong to the infrastructure that create the log (container_name, cluster_name, etc.). At least one attribute from
  attribute or resource is required 
  Logs that do not have at least one of these attributes will be dropped. 
  This is a safety net to help prevent accidentally adding dynamic labels that may significantly increase cardinality, 
  thus having a performance impact on your Loki instance. See the 
  [Loki label best practices](https://grafana.com/docs/loki/latest/best-practices/current-best-practices/) page for 
  additional details on the types of labels you may want to associate with log streams.

The following settings can be optionally configured:

- `tenant_id` (no default): The tenant ID used to identify the tenant the logs are associated to. This will set the 
  "X-Scope-OrgID" header used by Loki. If left unset, this header will not be added.


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


- `headers` (no default): Name/value pairs added to the HTTP request headers.

Example:

```yaml
loki:
  endpoint: http://loki:3100/loki/api/v1/push
  tenant_id: "example"
  labels:
    resource:
      # Allowing 'container.name' attribute and transform it to 'container_name', which is a valid Loki label name.
      container.name: "container_name"
      # Allowing 'k8s.cluster.name' attribute and transform it to 'k8s_cluster_name', which is a valid Loki label name.
      k8s.cluster.name: "k8s_cluster_name"
    attributes:
      # Allowing 'severity' attribute and not providing a mapping, since the attribute name is a valid Loki label name.
      severity: ""
      http.status_code: "http_status_code" 
      
  headers:
    "X-Custom-Header": "loki_rocks"
```

The full list of settings exposed for this exporter are documented [here](./config.go) with detailed sample
configurations [here](./testdata/config.yaml).

## Advanced Configuration

Several helper files are leveraged to provide additional capabilities automatically:

- [HTTP settings](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/confighttp/README.md)
- [Queuing and retry settings](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/exporterhelper/README.md)
