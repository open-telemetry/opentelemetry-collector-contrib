# Dynatrace Exporter

The [Dynatrace](https://dynatrace.com) metrics exporter exports metrics to the [metrics API v2](https://www.dynatrace.com/support/help/dynatrace-api/environment-api/metric-v2/post-ingest-metrics/)
using the [metrics ingestion protocol](https://www.dynatrace.com/support/help/how-to-use-dynatrace/metrics/metric-ingestion/metric-ingestion-protocol/).
This enables Dynatrace to receive metrics collected by the OpenTelemetry Collector.

The requests sent to Dynatrace are authenticated using an API token mechanism documented [here](https://www.dynatrace.com/support/help/dynatrace-api/basics/dynatrace-api-authentication/).

> Please review the Collector's [security
> documentation](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/security.md),
> which contains recommendations on securing sensitive information such as the
> API key required by this exporter.

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
        endpoint: 0.0.0.0:55680
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
