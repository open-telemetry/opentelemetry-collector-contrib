# Pulsar Exporter

Pulsar exporter exports logs, metrics, and traces to Pulsar. This exporter uses a synchronous producer
that blocks and able to batch messages.

Supported pipeline types: logs, metrics, traces

## Get Started

The following settings can be optionally configured:
- `service_url` (default = pulsar://localhost:6650): The url of pulsar cluster.
- `topic` (default = otlp_spans for traces, otlp_metrics for metrics, otlp_logs for logs): The name of the pulsar topic to export to.
- `encoding` (default = otlp_proto): The encoding of the traces sent to pulsar. All available encodings:
    - `otlp_proto`: payload is Protobuf serialized from `ExportTraceServiceRequest` if set as a traces exporter or `ExportMetricsServiceRequest` for metrics or `ExportLogsServiceRequest` for logs.
    - `otlp_json`:  ** EXPERIMENTAL ** payload is JSON serialized from `ExportTraceServiceRequest` if set as a traces exporter or `ExportMetricsServiceRequest` for metrics or `ExportLogsServiceRequest` for logs.
    - The following encodings are valid *only* for **traces**.
        - `jaeger_proto`: the payload is serialized to a single Jaeger proto `Span`, and keyed by TraceID.
        - `jaeger_json`: the payload is serialized to a single Jaeger JSON Span using `jsonpb`, and keyed by TraceID.
- `auth_name`: the auth type of `pulsar`. Permitted value: ``,`tls`,`token`,`athenz`,`oauth2`.
- `auth_param`: the parameter for auth.
- `tls_trust_certs_file_path`: path to the CA cert. For a client this verifies the server certificate. Should
  only be used if `insecure` is set to true.
- `insecure`: configure whether the Pulsar client accept untrusted TLS certificate from broker (default: false)
- `timeout` (default = 5s): Is the timeout for every attempt to send data to the backend.
- `retry_on_failure`
    - `enabled` (default = true)
    - `initial_interval` (default = 5s): Time to wait after the first failure before retrying; ignored if `enabled` is `false`
    - `max_interval` (default = 30s): Is the upper bound on backoff; ignored if `enabled` is `false`
    - `max_elapsed_time` (default = 120s): Is the maximum amount of time spent trying to send a batch; ignored if `enabled` is `false`
- `sending_queue`
    - `enabled` (default = true)
    - `num_consumers` (default = 10): Number of consumers that dequeue batches; ignored if `enabled` is `false`
    - `queue_size` (default = 5000): Maximum number of batches kept in memory before dropping data; ignored if `enabled` is `false`;
      User should calculate this as `num_seconds * requests_per_second` where:
        - `num_seconds` is the number of seconds to buffer in case of a backend outage
        - `requests_per_second` is the average number of requests per seconds.

Example configuration:
```yaml
exporters:
  pulsar:
    service_url: pulsar://localhost:6650
    topic: otlp-spans
    encoding: otlp_proto
    auth_name: tls
    auth_param: '{"tlsCertFile":"cert.pem","tlsKeyFile":"key.pem"}'
    insecure: false
    tls_trust_certs_file_path: ca.pem
```
