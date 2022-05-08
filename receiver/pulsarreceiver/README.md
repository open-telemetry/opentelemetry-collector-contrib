# Pulsar Receiver

Pulsar receiver receives logs, metrics, and traces from Pulsar.

Supported pipeline types: logs, metrics, traces

## Getting Started

The following settings can be optionally configured:
- `service_url` (default = pulsar://localhost:6650): The url of pulsar cluster.
- `topic` (default = otlp_spans for traces, otlp_metrics for metrics, otlp_logs for logs): The name of the pulsar topic to consume from.
- `encoding` (default = otlp_proto): The encoding of the payload sent to pulsar. Available encodings:
    - `otlp_proto`: the payload is deserialized to `ExportTraceServiceRequest`.
    - `jaeger_proto`: the payload is deserialized to a single Jaeger proto `Span`.
    - `jaeger_json`: the payload is deserialized to a single Jaeger JSON Span using `jsonpb`.
    - `zipkin_proto`: the payload is deserialized into a list of Zipkin proto spans.
    - `zipkin_json`: the payload is deserialized into a list of Zipkin V2 JSON spans.
    - `zipkin_thrift`: the payload is deserialized into a list of Zipkin Thrift spans.
- `consumer_name`: specifies the consumer name.
- `auth_name`: the auth type of `pulsar`. Permitted value: ``,`tls`,`token`,`athenz`,`oauth2`.
- `auth_param`: the parameter for auth.
- `subscription` (default = otlp_subscription): the subscription name of consumer.
- `tls_trust_certs_file_path`: path to the CA cert. For a client this verifies the server certificate. Should
  only be used if `insecure` is set to true.
- `insecure`: configure whether the Pulsar client accept untrusted TLS certificate from broker (default: false)


Example configuration:
```yaml
receivers:
  pulsar:
    service_url: pulsar://localhost:6650
    topic: otlp-spans
    subscription: otlp_spans_sub
    consumer_name: otlp_spans_sub_1
    encoding: otlp_proto
    auth_name: tls
    auth_param: '{"tlsCertFile":"cert.pem","tlsKeyFile":"key.pem"}'
    insecure: false
    tls_trust_certs_file_path: ca.pem
```
