# Elastic Observability Exporter

This exporter supports sending OpenTelemetry data to [Elastic Observability](https://www.elastic.co/observability).

Complete documentation is available on [Elastic.co](https://www.elastic.co/guide/en/apm/get-started/current/open-telemetry-elastic.html).

### Configuration options

- `apm_server_url` (required): Elastic APM Server URL.
- `api_key` (optional): credential for API Key authorization, if enabled in Elastic APM Server.
- `secret_token` (optional): credential for Secret Token authorization, if enabled in Elastic APM Server.
- `ca_file` (optional): root Certificate Authority (CA) certificate, for verifying the server's identity, if TLS is enabled.
- `cert_file` (optional): client TLS certificate.
- `key_file` (optional): client TLS key.
- `insecure` (optional): disable verification of the server's identity, if TLS is enabled.

### Example

```yaml
exporters:
    elastic:
        apm_server_url: "https://elasticapm.example.com"
        secret_token: "hunter2"
```
