# Simple Prometheus Receiver

### Overview

The `prometheus_simple` receiver is a wrapper around the [prometheus receiver](https://github.com/open-telemetry/opentelemetry-collector/tree/master/receiver/prometheusreceiver).
This receiver provides a simple configuration interface to configure the prometheus
receiver to scrape metrics from a single target. Here's an example config.

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


### Config

#### collection_interval

The internal at which metrics should be emitted by this receiver.

default: `10s`

#### endpoint

The endpoint from which prometheus metrics should be scraped.

default: `localhost:9090`

#### metrics_path

The path to the metrics endpoint.

default: `/metrics`

#### use_service_account

Whether or not to use the Kubernetes Pod service account for authentication.

default: `false`

#### tls_enabled

Whether or not to use TLS. Only if `tls_enabled` is set to `true`, the values under
`tls_config` are accounted for.

default: `false`


#### tls_config

Configure the below options in `tls_config`.

##### ca_file

Path to the CA cert that has signed the TLS certificate.

##### cert_file

Path to the client TLS certificate to use for TLS required connections.

##### key_file

Path to the client TLS key to use for TLS required connections.

##### insecure_skip_verify

Whether or not to skip certificate verification.

default: `false`