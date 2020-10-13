# Simple Prometheus Receiver

The `prometheus_simple` receiver is a wrapper around the [prometheus
receiver](https://github.com/open-telemetry/opentelemetry-collector/tree/master/receiver/prometheusreceiver).
This receiver provides a simple configuration interface to configure the
prometheus receiver to scrape metrics from a single target.

Supported pipeline types: metrics

## Configuration

The following settings are required:

- `endpoint` (default = `localhost:9090`): The endpoint from which prometheus
metrics should be scraped.

The following settings are optional:

- `collection_interval` (default = `10s`): The internal at which metrics should
be emitted by this receiver.
- `metrics_path` (default = `/metrics`): The path to the metrics endpoint.
- `use_service_account` (default = `false`): Whether or not to use the
Kubernetes Pod service account for authentication.
- `tls_enabled` (default = `false`): Whether or not to use TLS. Only if
`tls_enabled` is set to `true`, the values under `tls_config` are accounted
for.

The `tls_config` section supports the following options:

- `ca_file` (no default): Path to the CA cert that has signed the TLS
certificate.
- `cert_file` (no default): Path to the client TLS certificate to use for TLS
required connections.
- `key_file` (no default): Path to the client TLS key to use for TLS required
connections.
- `insecure_skip_verify` (default = `false`): Whether or not to skip
certificate verification.

Example:

```yaml
    receivers:
      prometheus_simple:
        collection_interval: 10s
        use_service_account: true
        endpoint: "172.17.0.5:9153"
        tls_enabled: true
        tls_config:
            ca_file: "/path/to/ca"
            cert_file: "/path/to/cert"
            key_file: "/path/to/key"
            insecure_skip_verify: true
    exporters:
      signalfx:
        access_token: <SIGNALFX_ACCESS_TOKEN>
        url: <SIGNALFX_INGEST_URL>

    service:
      pipelines:
        metrics:
          receivers: [prometheus_simple]
          exporters: [signalfx]
```

The full list of settings exposed for this receiver are documented [here](./config.go)
with detailed sample configurations [here](./testdata/config.yaml).
