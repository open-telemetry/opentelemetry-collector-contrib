# Pulsar Exporter

| Status                   |                       |
| ------------------------ |-----------------------|
| Stability                | [alpha]               |
| Supported pipeline types | traces, logs, metrics |
| Distributions            | [contrib]             |

Pulsar exporter exports logs, metrics, and traces to Pulsar. This exporter uses a synchronous producer
that blocks and able to batch messages.

Supported pipeline types: logs, metrics, traces

## Get Started

The following settings can be optionally configured:
- `endpoint` (default = pulsar://localhost:6650): The url of pulsar cluster.
- `topic` (default = otlp_spans for traces, otlp_metrics for metrics, otlp_logs for logs): The name of the pulsar topic to export to.
- `encoding` (default = otlp_proto): The encoding of the traces sent to pulsar. All available encodings:
    - `otlp_proto`: payload is Protobuf serialized from `ExportTraceServiceRequest` if set as a traces exporter or `ExportMetricsServiceRequest` for metrics or `ExportLogsServiceRequest` for logs.
    - `otlp_json`:  ** EXPERIMENTAL ** payload is JSON serialized from `ExportTraceServiceRequest` if set as a traces exporter or `ExportMetricsServiceRequest` for metrics or `ExportLogsServiceRequest` for logs.
    - The following encodings are valid *only* for **traces**.
        - `jaeger_proto`: the payload is serialized to a single Jaeger proto `Span`, and keyed by TraceID.
        - `jaeger_json`: the payload is serialized to a single Jaeger JSON Span using `jsonpb`, and keyed by TraceID.
- `auth`
    - `tls`
        - `cert_file`:
        - `key_file`:
    - `token`
        - `token`
    - `oauth2`
        - `issuer_url`:
        - `client_id`:
        - `audience`:
    - `athenz`
        - `provider_domain`:
        - `tenant_domain`:
        - `tenant_service`:
        - `private_key`:
        - `key_id`:
        - `principal_header`:
        - `zts_url`:
- `tls_trust_certs_file_path`: path to the CA cert. For a client this verifies the server certificate. Should
  only be used if `insecure` is set to true.
- `tls_allow_insecure_connection`: configure whether the Pulsar client accept untrusted TLS certificate from broker (default: false)
- `timeout`: send pulsar message timeout (default: 5s)
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
    auth:
      tls:
        cert_file: cert.pem
        key_file: key.pem
    timeout: 10s
    tls_allow_insecure_connection: false
    tls_trust_certs_file_path: ca.pem
```

[alpha]:https://github.com/open-telemetry/opentelemetry-collector#alpha
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib

