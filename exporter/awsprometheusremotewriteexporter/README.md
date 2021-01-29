# AWS Prometheus Remote Write Exporter

This Exporter sends metrics data in Prometheus TimeSeries format to a Prometheus Remote Write Backend and signs each outgoing HTTP request following
the AWS Signature Version 4 signing process. AWS region and service must be provided in the configuration file, and AWS
credentials are retrieved from the [default credential chain](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials)
of the AWS SDK for Go.

Note: this exporter imports and uses the [Prometheus remote write exporter](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/prometheusremotewriteexporter)
from upstream, and simply wraps it in Sigv4 authentication logic

Same as the Prometheus remote write exporter, this exporter checks the temporality and the type of each incoming metric 
and only exports the following combination:

- Int64 or Double type with any temporality
- MonotonicInt64, MonotonicDouble, Histogram, or Summary with only Cumulative temporality.

## Configuration
The following settings are required:
- `endpoint`: protocol:host:port to which the exporter is going to send traces or metrics, using the HTTP/HTTPS protocol. 

The following settings can be optionally configured:
- `namespace`: prefix attached to each exported metric name.
- `headers`: additional headers attached to each HTTP request. If `X-Prometheus-Remote-Write-Version` is set by user, its value must be `0.1.0`
- `insecure` (default = false): whether to enable client transport security for the exporter's connection.
- `ca_file`: path to the CA cert. For a client this verifies the server certificate. Should only be used if `insecure` is set to true.
- `cert_file`: path to the TLS cert to use for TLS required connections. Should only be used if `insecure` is set to true.
- `key_file`: path to the TLS key to use for TLS required connections. Should only be used if `insecure` is set to true.
- `timeout` (default = 5s): How long to wait until the connection is close.
- `read_buffer_size` (default = 0): ReadBufferSize for HTTP client.
- `write_buffer_size` (default = 512 * 1024): WriteBufferSize for HTTP client.
- `aws_auth`: specify if each request should be signed with AWS Sig v4. The following settings must be configured:
    - `region`: region of the AWS service being exported to.
    - `service`: AWS service being exported to.
    
    
#### Examples:

Simplest configuration:
```yaml
exporters:
  awsprometheusremotewrite:
    endpoint: "http://some.url:9411/api/prom/push"
```

All configurations:
```yaml
exporters:
  awsprometheusremotewrite:
    namespace: "test-space"
    sending_queue:
        enabled: true
        num_consumers: 2
        queue_size: 10
    retry_on_failure:
        enabled: true
        initial_interval: 10s
        max_interval: 60s
        max_elapsed_time: 10m
    endpoint: "http://localhost:9009"
    ca_file: "/var/lib/mycert.pem"
    write_buffer_size: 524288
    headers:
        Prometheus-Remote-Write-Version: "0.1.0"
        X-Scope-OrgID: 234
    aws_auth:
        region: "us-west-2"
        service: "service-name"
    external_labels:
        key1: value1
        key2: value2
```

The full list of settings exposed for this exporter are documented [here](./config.go)
with detailed sample configurations [here](./testdata/config.yaml).
