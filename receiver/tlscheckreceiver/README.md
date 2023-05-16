# SSH Check Receiver

| Status                   |           |
| ------------------------ |-----------|
| Stability                | [in development] |
| Supported pipeline types | metrics   |
| Distributions            | [contrib] |

The TLS Check Receiver propose to check protocol versions, certificate expiration, validity, and other statuses. It is
crucial to have visibility into certificate expiration and other TLS-related issues to ensure the security of our systems.
We propose the addition of a new TLS Check Receiver that will allow users to monitor and receive alerts on their TLS certificates' status.

## Configuration

The following settings are required:
- `endpoint`
- `collection_interval` (default = `60s`): This receiver collects metrics on an interval. Valid time units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`.
The following settings are optional:

### TLS settings
- `ca_file` (optional): Root Certificate Authority (CA) certificate, for verifying the server's identity, if TLS is enabled.
- `cert_file` (optional): Client TLS certificate.
- `key_file` (optional): Client TLS key.
- `insecure` (optional): In gRPC when set to true, this is used to disable the client transport security. In HTTP, this disables verifying the server's certificate chain and host name.
- `insecure_skip_verify` (optional): Will enable TLS but not verify the certificate.

### Example Configuration

```yaml
receivers:
  tlscheck:
    endpoint: localhost:8443
    tls:
      insecure_skip_verify: true
    collection_interval: 10s
```

The full list of settings exposed for this receiver are documented [here](./config.go) with detailed sample configurations [here](./testdata/config.yaml).

## Metrics

Details about the metrics produced by this receiver can be found in [metadata.yaml](./metadata.yaml)

[beta]: https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
