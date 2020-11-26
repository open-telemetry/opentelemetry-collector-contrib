# Dynatrace Exporter

This exporter sends metric data to [Dynatrace](https://dynatrace.com).

## Configuration

A Dynatrace API Token and metrics ingest endpoint are required.

 ```yaml
dynatrace:
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
    api_token: my_api_token

service:
  pipelines:
    metrics:
      receivers: [prometheus]
      processors: [batch]
      exporters: [dynatrace]

 ```
