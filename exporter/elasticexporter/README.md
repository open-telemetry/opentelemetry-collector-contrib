# Elastic native support for OpenTelemetry

ℹ️ Since version 7.13, Elastic supports native OTLP ingestion. This means you can use an [OTLP exporter](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/otlpexporter) to send data to Elastic, instead of the OpenTelemetry Collector Exporter for Elastic. **The `otlp` exporter is the recommended way to integrate the OpenTelemetry Collector to Elastic.**

For more details, see the [Elastic documentation to integrate with OpenTelemetry](https://www.elastic.co/guide/en/apm/get-started/current/open-telemetry-elastic.html).

## Sample configurations

### Sample configuration using an Elastic APM Secret Token

When authenticating with an [Elastic APM Secret Token](https://www.elastic.co/guide/en/apm/server/current/secret-token.html), define an `Authorization: "Bearer xxx"` header on the OTLP exporter:

```yaml
...
exporters:
  otlp/elastic:
      endpoint: "xxx.elastic-cloud.com:443"
      headers:
          Authorization: "Bearer your-apm-secret-token"
service:
  pipelines:
    metrics:
      receivers:
        - otlp
      processors:
        - batch
      exporters:
        - otlp/elastic
    traces:
      receivers:
        - otlp
      processors:
        - batch
      exporters:
        - otlp/elastic
```

### Sample configuration using an Elastic API Key

When authenticating with an [Elastic API Key](https://www.elastic.co/guide/en/apm/server/current/api-key.html), define an `Authorization: "ApiKey xxx"` header on the OTLP exporter:

```yaml
exporters:
  otlp/elastic:
      endpoint: "xxx.elastic-cloud.com:443"
      headers:
          Authorization: "ApiKey your-api-key"
...
```

### Sample configuration for an insecure setup disabling TLS and authentication

```yaml
exporters:
  otlp/elastic:
      endpoint: "localhost:8200"
      tls:
        insecure: true
```

## Migration

ℹ️ The native support of OpenTelemetry by Elastic doesn't remove the architectural benefits of using the OpenTelemetry Collector in observability architectures.
The OpenTelemetry Collector continues to add high availability, scalability, retries, live reconfiguration (like with sampling), data enrichment, ingestion of various protocols such as Jaeger or Zipkin, etc.

To migrate from the legacy OpenTelemetry Collector exporter for Elastic to the native support of OpenTelemetry in Elastic, replace the OpenTelemetry Collector's `elastic` exporter with an `otlp` exporter.

Sample migration:

```yaml
...
exporters:

  ## REMOVE THE DEFINITION OF THE `ELASTIC` EXPORTER
  # elastic:
  #      apm_server_url: "https://elasticapm.example.com"
  #      secret_token: "hunter2"

  ## INTRODUCE THE DEFINITION OF AN `OTLP` EXPORTER (SAME ELASTIC HOST, SAME AUTHENTICATION TOKEN OR KEY, DON'T FORGET TO SPECIFY THE LISTEN PORT)
  otlp/elastic:
      endpoint: "xxx.elastic-cloud.com:443"
      headers:
          Authorization: "Bearer hunter2"
service:
  pipelines:
    metrics:
      receivers:
        - otlp
      processors:
        - batch
      exporters:
      
        ## REMOVE THE `ELASTIC` EXPORTER
        # - elastic
        
        ## ADD THE `OTLP` EXPORTER
        - otlp/elastic
    traces:
      receivers:
        - otlp
      processors:
        - batch
      exporters:
      
        ## REMOVE THE `ELASTIC` EXPORTER
        # - elastic
        
        ## ADD THE `OTLP` EXPORTER
        - otlp/elastic
```


# Legacy OpenTelemetry Collector Exporter for Elastic


This exporter supports sending OpenTelemetry data to [Elastic Observability](https://www.elastic.co/observability).

Complete documentation is available on [Elastic.co](https://www.elastic.co/guide/en/apm/get-started/current/open-telemetry-elastic.html).

### Configuration options

- `apm_server_url` (required): Elastic APM Server URL.
- `api_key` (optional): credential for API Key authorization, if enabled in Elastic APM Server.
- `secret_token` (optional): credential for Secret Token authorization, if enabled in Elastic APM Server.
- `tls:`
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
