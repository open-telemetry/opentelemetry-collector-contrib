# Dynatrace Exporter

The [Dynatrace](https://dynatrace.com) metrics exporter exports metrics to the [metrics API v2](https://www.dynatrace.com/support/help/dynatrace-api/environment-api/metric-v2/post-ingest-metrics/)
using the [metrics ingestion protocol](https://www.dynatrace.com/support/help/how-to-use-dynatrace/metrics/metric-ingestion/metric-ingestion-protocol/).
This enables Dynatrace to receive metrics collected by the OpenTelemetry Collector.

The requests sent to Dynatrace are authenticated using an API token mechanism documented [here](https://www.dynatrace.com/support/help/dynatrace-api/basics/dynatrace-api-authentication/).

> Please review the Collector's [security
> documentation](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/security.md),
> which contains recommendations on securing sensitive information such as the
> API key required by this exporter.

## Requirements

You will either need a Dynatrace OneAgent (version 1.201 or higher) installed on the same host as the Collector; or a Dynatrace environment with version 1.202 or higher.

- Collector contrib minimum version: 0.18.0


## Getting Started

The Dynatrace exporter is enabled by adding a `dynatrace` entry to the `exporters` section of your config file.
All configurations are optional, but if an `endpoint` other than the OneAgent metric ingestion endpoint is specified then an `api_token` is required.
To see all available options, see [Advanced Configuration](#advanced-configuration) below.

### Running alongside Dynatrace OneAgent (preferred)

If you run the Collector on a host or VM that is monitored by the Dynatrace OneAgent then you only need to enable the exporter. No further configurations needed. The Dynatrace exporter will send all metrics to the OneAgent which will use its secure and load balanced connection to send the metrics to your Dynatrace SaaS or Managed environment.
Depending on your environment, you might have to enable metrics ingestion on the OneAgent first as described in the [Dynatrace documentation](https://www.dynatrace.com/support/help/how-to-use-dynatrace/metrics/metric-ingestion/ingestion-methods/opentelemetry/).

> Note: The name and identifier of the host running the Collector will be added as a dimension to every metric. If this is undesirable, then the output plugin may be used in standalone mode using the directions below.

```yaml
exporters:
  dynatrace:
    ## No options are required. By default, metrics will be exported via the OneAgent on the local host.
```

### Running standalone

If you run the Collector on a host or VM without a OneAgent you will need to configure the Metrics v2 API endpoint of your Dynatrace environment to send the metrics to as well as an API token.

Find out how to create a token in the [Dynatrace documentation](https://www.dynatrace.com/support/help/dynatrace-api/basics/dynatrace-api-authentication/) or navigate to **Settings > Integration > Dynatrace API** in your Dynatrace environment and create a token with the 'Ingest metrics' (`metrics.ingest`) scope enabled. It is recommended to limit token scope to only this permission.

The endpoint for the Dynatrace Metrics API v2 is:

* on Dynatrace Managed: `https://{your-domain}/e/{your-environment-id}/api/v2/metrics/ingest`
* on Dynatrace SaaS: `https://{your-environment-id}.live.dynatrace.com/api/v2/metrics/ingest`

More details can be found in the [Dynatrace documentation for the Metrics v2 API](https://www.dynatrace.com/support/help/dynatrace-api/environment-api/metric-v2/post-ingest-metrics/).

```yaml
exporters:
  dynatrace:
    ## If no OneAgent is running on the host, endpoint and api_token need to be set

    ## Dynatrace Metrics Ingest v2 endpoint to receive metrics
    endpoint: "https://{your-environment-id}.live.dynatrace.com/api/v2/metrics/ingest"

    ## API token is required if an endpoint is specified and should be restricted to the 'Ingest metrics' scope
    ## hard-coded for illustration only, should be read from a secure source
    api_token: "your API token here" 
```

You can learn more about how to use the Dynatrace API [here](https://www.dynatrace.com/support/help/dynatrace-api/).

### Metric Batching

Dynatrace recommends the use of the batch processor with a maximum batch size of 1000 metrics and a timeout between 10 and 60 seconds. Batches with more than 1000 metrics may be throttled by Dynatrace.

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
    # Batch size must be less than or equal to 1000
    send_batch_max_size: 1000
    timeout: 30s

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

- [HTTP settings](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/confighttp/README.md)
- [TLS and mTLS settings](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/configtls/README.md)
- [Queuing, retry and timeout settings](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/exporterhelper/README.md) except timeout which is handled by the HTTP settings

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
exporters:
  dynatrace:
    endpoint: https://ab12345.live.dynatrace.com
    api_token: <api token must have metrics.write permission>
    tags:
      - example=tag
    prefix: my_prefix
    headers:
      - header1: value1
    read_buffer_size: 4000
    write_buffer_size: 4000
    timeout: 30s
    insecure_skip_verify: false
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 120s
    sending_queue:
      enabled: true
      num_consumers: 10
      queue_size: 5000
service:
  extensions:
  pipelines:
    metrics:
      receivers: [otlp]
      exporters: [dynatrace]
```

### tags (Optional)

Tags are included as dimensions on all exported metrics.
Tags must be in the `key=value` dimension format specified by the [metrics ingestion protocol](https://www.dynatrace.com/support/help/how-to-use-dynatrace/metrics/metric-ingestion/metric-ingestion-protocol/).

### prefix (Optional)

Prefix is a string which will be used as the first part of a dot-separated metric key.
For example, if a metric with name `request_count` is prefixed with `my_service`, the resulting
metric key is `my_service.request_count`.

### headers (Optional)

Additional headers to be included with every outgoing http request.

### read_buffer_size (Optional)

Defines the buffer size to allocate to the HTTP client for reading the response.

Default: `4096`

### write_buffer_size (Optional)

Defines the buffer size to allocate to the HTTP client for writing the payload

Default: `4096`

### timeout (Optional)

Timeout specifies a time limit for requests made by this
Client. The timeout includes connection time, any
redirects, and reading the response body. The timer remains
running after Get, Head, Post, or Do return and will
interrupt reading of the Response.Body.

https://golang.org/pkg/net/http/#Client

Default: `0`

### insecure_skip_verify (Optional)

Additionally you can configure TLS to be enabled but skip verifying the server's certificate chain. This cannot be combined with insecure since insecure won't use TLS at all.

Default: `false`

### retry_on_failure.enabled (Optional)

Default: `true`

### retry_on_failure.initial_interval (Optional)

Time to wait after the first failure before retrying; ignored if enabled is false.

Default: `5s`

### retry_on_failure.max_interval (Optional)

The upper bound on backoff; ignored if enabled is false

Default: `30s`

### retry_on_failure.max_elapsed_time (Optional)

The maximum amount of time spent trying to send a batch; ignored if enabled is false.

Default: `120s`

### sending_queue.enabled (Optional)

Default: `true`

### sending_queue.num_consumers (Optional)

Number of consumers that dequeue batches; ignored if enabled is false.

Default: `10`

### sending_queue.queue_size (Optional)

Maximum number of batches kept in memory before data; ignored if enabled is false;
User should calculate this as `num_seconds * requests_per_second` where:

- `num_seconds` is the number of seconds to buffer in case of a backend outage
- `requests_per_second` is the average number of requests per seconds.

Default: `5000`
