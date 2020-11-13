# Dynatrace Exporter

This exporter sends metric data to [Dynatrace](https://dynatrace.com).

## Configuration

The only required setting is a Dynatrace API token.
 ```yaml
dynatrace:
  api:
    api_token: "my_api_token"
 ```
 
 To specify a Dynatrace metrics ingest endpoint, use the `api.url` configuration.

 ```yaml
datadog:
  api:
    api_token: "my_api_token"
    url: https://abc12345.live.dynatrace.com/api/v2/metrics/ingest
 ```

Example:

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
    tags:
      - example=tag

    api:
      url: https://abc12345.live.dynatrace.com/api/v2/metrics/ingest
      api_token: my_api_token

service:
  pipelines:
    metrics:
      receivers: [prometheus]
      processors: [batch]
      exporters: [dynatrace]

 ```
