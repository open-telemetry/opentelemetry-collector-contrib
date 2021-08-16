# Google Cloud Exporter

This exporter can be used to send metrics and traces to Google Cloud Monitoring and Trace (formerly known as Stackdriver) respectively.

## Getting started

These instructions are to get you up and running quickly with the GCP exporter in a local development environment. We'll also point out alternatives that may be more suitable for CI or production.

1.  **Obtain a binary.** Pull a Docker image for the OpenTelemetry contrib collector, which includes the GCP exporter plugin.

    ```sh
    docker pull otel/opentelemetry-collector-contrib
    ```

    <details>
    <summary>Alternatives</summary>

    *   Download a [binary or package of the OpenTelemetry Collector](https://github.com/open-telemetry/opentelemetry-collector-contrib/releases) that is appropriate for your platform, and includes the Google Cloud exporter.
    *   Create your own main package in Go, that pulls in just the plugins you need.
    *   Use the [OpenTelemetry Collector Builder](https://github.com/open-telemetry/opentelemetry-collector-builder) to generate the Go main package and `go.mod`.

    </details>
    

2.  **Create a configuration file `config.yaml`.** The example below shows a minimal recommended configuration that receives OTLP and sends data to GCP, in addition to verbose logging to help understand what is going on. It uses application default credentials (which we will set up in the next step).

    Note that this configuration includes the recommended `memory_limiter` and `batch` plugins, which avoid high latency for reporting telemetry, and ensure that the collector itself will stay stable (not run out of memory) by dropping telemetry if needed.

    ```yaml
    receivers:
      otlp:
        protocols:
          grpc:
          http:
    exporters:
      googlecloud:
      logging:
        loglevel: debug
    processors:
      memory_limiter:
      batch:
    service:
      pipelines:
        traces:
          receivers: [otlp]
          processors: [memory_limiter, batch]
          exporters: [googlecloud, logging]
        metrics:
          receivers: [otlp]
          processors: [memory_limiter, batch]
          exporters: [googlecloud, logging]
    ```

3.  **Set up credentials.**

    1.  Enable billing in your GCP project.

    2.  Enable the Cloud Metrics and Cloud Trace APIs.

    3.  Ensure that your user GCP user has (at minimum) `roles/monitoring.metricWriter` and `roles/cloudtrace.agent`. You can learn about [metric-related](https://cloud.google.com/monitoring/access-control) and [trace-related](https://cloud.google.com/trace/docs/iam) IAM in the GCP documentation.

    4.  Obtain credentials.

        ```sh
        gcloud auth application-default login
        ```

    <details>
      <summary>Alternatives</summary>

      * You can run the collector as a service account, as long as it has the necessary roles. This is useful in production, because credentials for a user are short-lived.

      * You can also run the collector on a GCE VM or as a GKE workload, which will use the service account associated with GCE/GKE.
    </details>

4.  **Run the collector.** The following command mounts the configuration file and the credentials as Docker volumes. It runs the collector in the foreground, so please execute it in a separate terminal.

    ```sh
    docker run \
      --volume ~/.config/gcloud/application_default_credentials.json:/etc/otel/key.json \
      --volume $(pwd)/config.yaml:/etc/otel/config.yaml \
      --env GOOGLE_APPLICATION_CREDENTIALS=/etc/otel/key.json \
      --expose 4317 \
      --expose 55681 \
      --rm \
      otel/opentelemetry-collector-contrib
    ```

    <details>
    <summary>Alternatives</summary>

    If you obtained OS-specific packages or built your own binary in step 1, you'll need to follow the appropriate conventions for running the collector.

    </details>

5.  **Gather telemetry.** Run an application that can submit OTLP-formatted metrics and traces, and configure it to send them to `127.0.0.1:4317` (for gRPC) or `127.0.0.1:55681` (for HTTP).

    <details>
      <summary>Alternatives</summary>

      *   Set up the host metrics receiver, which will gather telemetry from the host without needing an external application to submit telemetry.

      *   Set up an application-specific receiver, such as the Nginx receiver, and run the corresponding application.

      *   Set up a receiver for some other protocol (such Prometheus, StatsD, Zipkin or Jaeger), and run an application that speaks one of those protocols.
    </details>

6.  **View telemetry in GCP.** Use the GCP [metrics explorer](https://console.cloud.google.com/monitoring/metrics-explorer) and [trace overview](https://console.cloud.google.com/traces) to view your newly submitted telemetry.

## Configuration reference

The following configuration options are supported:

- `project` (optional): GCP project identifier.
- `endpoint` (optional): Endpoint where data is going to be sent to.
- `user_agent` (optional): Override the user agent string sent on requests to Cloud Monitoring (currently only applies to metrics). Specify `{{version}}` to include the application version number. Defaults to `opentelemetry-collector-contrib {{version}}`.
- `use_insecure` (optional): If true. use gRPC as their communication transport. Only has effect if Endpoint is not "".
- `timeout` (optional): Timeout for all API calls. If not set, defaults to 12 seconds.
- `number_of_workers` (optional): NumberOfWorkers sets the number of go rountines that send requests. The minimum number of workers is 1.
- `resource_mappings` (optional): ResourceMapping defines mapping of resources from source (OpenCensus) to target (Google Cloud).
  - `label_mappings` (optional): Optional flag signals whether we can proceed with transformation if a label is missing in the resource.
- `retry_on_failure` (optional): Configuration for how to handle retries when sending data to Google Cloud fails.
  - `enabled` (default = true)
  - `initial_interval` (default = 5s): Time to wait after the first failure before retrying; ignored if `enabled` is `false`
  - `max_interval` (default = 30s): Is the upper bound on backoff; ignored if `enabled` is `false`
  - `max_elapsed_time` (default = 120s): Is the maximum amount of time spent trying to send a batch; ignored if `enabled` is `false`
- `sending_queue` (optional): Configuration for how to buffer traces before sending.
  - `enabled` (default = true)
  - `num_consumers` (default = 10): Number of consumers that dequeue batches; ignored if `enabled` is `false`
  - `queue_size` (default = 5000): Maximum number of batches kept in memory before data; ignored if `enabled` is `false`;
    User should calculate this as `num_seconds * requests_per_second` where:
    - `num_seconds` is the number of seconds to buffer in case of a backend outage
    - `requests_per_second` is the average number of requests per seconds.

Note: These `retry_on_failure` and `sending_queue` are provided (and documented) by the [Exporter Helper](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/exporterhelper#configuration)

Additional configuration for the metric exporter:

- `metric.prefix` (optional): MetricPrefix overrides the prefix / namespace of the Google Cloud metric type identifier. If not set, defaults to "custom.googleapis.com/opencensus/"
- `metric.skip_create_descriptor` (optional): Whether to skip creating the metric descriptor.

Example:

```yaml
exporters:
  googlecloud:
    project: my-project
    endpoint: test-endpoint
    user_agent: my-collector {{version}}
    use_insecure: true
    timeout: 12s
    number_of_workers: 3

    resource_mappings:
      - source_type: source.resource1
        target_type: target-resource1
        label_mappings:
          - source_key: contrib.opencensus.io/exporter/googlecloud/project_id
            target_key: project_id
            optional: true
          - source_key: source.label1
            target_key: target_label_1

    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 120s
    sending_queue:
      enabled: true
      num_consumers: 2
      queue_size: 50

    metric:
      prefix: prefix
      skip_create_descriptor: true
```

Beyond standard YAML configuration as outlined in the sections that follow,
exporters that leverage the net/http package (all do today) also respect the
following proxy environment variables:

* HTTP_PROXY
* HTTPS_PROXY
* NO_PROXY

If set at Collector start time then exporters, regardless of protocol,
will or will not proxy traffic as defined by these environment variables.


## Recommendations

It is recommended to always run a [batch processor](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/batchprocessor)
and [memory limiter](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/memorylimiter) for tracing pipelines to ensure
optimal network usage and avoiding memory overruns.  You may also want to run an additional
[sampler](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/probabilisticsamplerprocessor), depending on your needs.


## Deprecatations

The previous trace configuration (v0.21.0) has been deprecated in favor of the common configuration options available in OpenTelemetry. These will cause a failure to start
and should be migrated:

- `trace.bundle_delay_threshold` (optional): Use `batch` processor instead ([docs](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/batchprocessor)).
- `trace.bundle_count_threshold` (optional): Use `batch` processor instead ([docs](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/batchprocessor)).
- `trace.bundle_byte_threshold` (optional): Use `memorylimiter` processor instead ([docs](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/memorylimiter))
- `trace.bundle_byte_limit` (optional): Use `memorylimiter` processor instead ([docs](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/memorylimiter))
- `trace.buffer_max_bytes` (optional): Use `memorylimiter` processor instead ([docs](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/memorylimiter))
