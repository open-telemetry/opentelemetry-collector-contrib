# Dynatrace Exporter

This exporter sends metric data to [Dynatrace](https://dynatrace.com).

## Configuration

A Dynatrace API Token and metrics ingest endpoint are required.

Creating an API token for your Dynatrace environment is described in the [Dynatrace API documentation](https://www.dynatrace.com/support/help/dynatrace-api/basics/dynatrace-api-authentication/).
The only access scope required for exporting metrics is the **Ingest metrics** (`metrics.ingest`) scope listed in the **API v2** section.

Given an environment ID `abc12345` on Dynatrace SaaS, the [metrics ingest endpoint](https://www.dynatrace.com/support/help/dynatrace-api/environment-api/metric-v2/post-ingest-metrics/) would be `https://abc12345.live.dynatrace.com/api/v2/metrics/ingest`.

 ```yaml
dynatrace:
  # Token must at least have the Ingest metrics (metrics.ingest) permission
  api_token: "my_api_token"
  endpoint: https://abc12345.live.dynatrace.com/api/v2/metrics/ingest
 ```

Full example:

 ```yaml
receivers:
  # Collect own metrics and export them to Dynatrace
  prometheus:
    config:
      scrape_configs:
        - job_name: 'otel-collector'
          scrape_interval: 10s
          static_configs:
            - targets: [ '0.0.0.0:8888' ]

processors:
  batch:

exporters:
  dynatrace:
    # optional - tags specified here will be included as a dimension on every exported metric
    tags:
      - example=tag

    # optional - prefix will be prepended to each metric name in prefix.name format
    prefix: my_prefix

    endpoint: https://abc12345.live.dynatrace.com/api/v2/metrics/ingest
    # Token must at least have the Ingest metrics (metrics.ingest) permission
    api_token: my_api_token

service:
  pipelines:
    metrics:
      receivers: [prometheus]
      processors: [batch]
      exporters: [dynatrace]

 ```

## Advanced Configuration

Several helper files are leveraged to provide additional capabilities automatically:

- [HTTP settings](https://github.com/open-telemetry/opentelemetry-collector/blob/master/config/confighttp/README.md)
- [TLS and mTLS settings](https://github.com/open-telemetry/opentelemetry-collector/blob/master/config/configtls/README.md)
- [Queuing, retry and timeout settings](https://github.com/open-telemetry/opentelemetry-collector/blob/master/exporter/exporterhelper/README.md) except timeout which is handled by the HTTP settings
