# AWS Prometheus Remote Write Exporter

AWS Prometheus Remote Write Exporter sends metrics in remote write format to
[Amazon Managed Service for Prometheus](https://aws.amazon.com/prometheus/).
The exporter uses AWS Signature Version 4 signing process for authentication
and uses credentials from the 
[default credential chain](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials).

Note: This exporter is similar to [Prometheus remote write exporter](../prometheusremotewriteexporter)
and it only adds SigV4 support to it.

Similar to the Prometheus remote write exporter, the exporter checks the
temporality and the type of each incoming metric
and only exports the following combination:

- Int64 or Double type with any temporality
- MonotonicInt64, MonotonicDouble, Histogram, or Summary with only Cumulative temporality.

## Configuration

The following settings are required:

- `endpoint`: protocol:host:port to which the exporter is going to send traces or metrics, using the HTTP/HTTPS protocol.

The following settings can be optionally configured:

- `namespace`: prefix attached to each exported metric name.
- `headers`: additional headers attached to each HTTP request. If `X-Prometheus-Remote-Write-Version` is set by user, its value must be `0.1.0`
- `tls`:
  - `insecure` (default = false): whether to enable client transport security for the exporter's connection.
  - `ca_file`: path to the CA cert. For a client this verifies the server certificate. Should only be used if `insecure` is set to true.
  - `cert_file`: path to the TLS cert to use for TLS required connections. Should only be used if `insecure` is set to true.
  - `key_file`: path to the TLS key to use for TLS required connections. Should only be used if `insecure` is set to true.
- `timeout` (default = 5s): How long to wait until the connection is close.
- `read_buffer_size` (default = 0): ReadBufferSize for HTTP client.
- `write_buffer_size` (default = 512 * 1024): WriteBufferSize for HTTP client.
- `aws_auth`: specify if each request should be signed with AWS Sig v4. The following settings must be configured:
    - `region`: region of the AWS service being exported to.
    - `role_arn`: Amazon Resource Name of the role to assume.

### Examples

Simplest configuration:

```yaml
exporters:
  awsprometheusremotewrite:
    endpoint: "https://aps-workspaces.us-east-1.amazonaws.com/workspaces/ws-XXX/api/v1/remote_write"
```

All configurations:

```yaml
exporters:
  awsprometheusremotewrite:
    namespace: "test-space"
    retry_on_failure:
        enabled: true
        initial_interval: 10s
        max_interval: 60s
        max_elapsed_time: 10m
    endpoint: "https://aps-workspaces.us-east-1.amazonaws.com/workspaces/ws-XXX/api/v1/remote_write"
    aws_auth:
        region: "us-east-1" # need to match workspace region
        service: "aps"
        role_arn: "arn:aws:iam::123456789012:role/aws-service-role/access"
    ca_file: "/var/lib/mycert.pem"
    write_buffer_size: 524288
    external_labels:
        key1: value1
        key2: value2
```

The full list of settings exposed for this exporter are documented [here](./config.go)
with detailed sample configurations [here](./testdata/config.yaml).
