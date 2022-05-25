# Google Cloud Exporter

| Status                   |                    |
| ------------------------ |--------------------|
| Stability                | [beta]             |
| Supported pipeline types | trace, metrics |
| Distributions            | [contrib]          |

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
    *   Use the [OpenTelemetry Collector Builder](https://github.com/open-telemetry/opentelemetry-collector/tree/main/cmd/builder) to generate the Go main package and `go.mod`.

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
        # Google Cloud Monitoring returns an error if any of the points are invalid, but still accepts the valid points.
        # Retrying successfully sent points is guaranteed to fail because the points were already written.
        # This results in a loop of unnecessary retries.  For now, disable retry_on_failure.
        retry_on_failure:
          enabled: false
      logging:
        loglevel: debug
    processors:
      memory_limiter:
          check_interval: 1s
          limit_mib: 4000
          spike_limit_mib: 800
      batch:
        # Google Cloud Monitoring limits batches to 200 metric points.
        send_batch_max_size: 200
        send_batch_size: 200
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
      -p 4317:4317 \
      -p 4318:4318 \
      -p 55681:55681 \
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
- `user_agent` (optional): Override the user agent string sent on requests to Cloud Monitoring (currently only applies to metrics). Specify `{{version}}` to include the application version number. Defaults to `opentelemetry-collector-contrib {{version}}`.
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

- `metric.endpoint` (optional): Endpoint where metric data is going to be sent to. Replaces `endpoint`.
- `metric.use_insecure` (optional): If true, use gRPC as their communication transport. Only has effect if Endpoint is not "".
- `metric.known_domains` (optional): If a metric belongs to one of these domains it does not get a prefix.
- `metric.skip_create_descriptor` (optional): If set to true, do not send metric descriptors to GCM.
- `metric.instrumentation_library_labels` (optional): If true, set the instrumentation_source and instrumentation_version labels. Defaults to true.
- `metric.create_service_timeseries` (optional): If true, this will send all timeseries using `CreateServiceTimeSeries`. Implicitly, this sets `skip_create_descriptor` to true.
- `metric.create_metric_descriptor_buffer_size` (optional): Buffer size for the channel which asynchronously calls CreateMetricDescriptor. Default is 10.
- `metric.service_resource_labels` (optional):  If true, the exporter will copy OTel's service.name, service.namespace, and service.instance.id resource attributes into the GCM timeseries metric labels. Default is true.
- `metric.resource_filters` (optional): If provided, resource attributes matching any filter will be included in metric labels. Defaults to empty, which won't include any additional resource labels.
  - `prefix`: Match resource keys by prefix

Additional configuration added for the trace exporter:

- `trace.endpoint` (optional): Endpoint where trace data is going to be sent to. Replaces `endpoint`.
- `trace.use_insecure` (optional): If true. use gRPC as their communication transport. Only has effect if Endpoint is not "". Replaces `use_insecure`.
- `trace.attribute_mappings` (optional): AttributeMappings determines how to map from OpenTelemetry attribute keys to Google Cloud Trace keys.  By default, it changes http and service keys so that they appear more prominently in the UI.

Additional configuration for the logging exporter:

- `log.default_log_name` (optional): Defines a default name for log entries. If left unset, and a log entry does not have the `gcp.log_name` 
attribute set, the exporter will return an error processing that entry.

Example:

```yaml
exporters:
  googlecloud:
    # Google Cloud Monitoring returns an error if any of the points are invalid, but still accepts the valid points.
    # Retrying successfully sent points is guaranteed to fail because the points were already written.
    # This results in a loop of unnecessary retries.  For now, disable retry_on_failure.
    retry_on_failure:
      enabled: false
    project: my-project
    user_agent: my-collector {{version}}
    trace:
      endpoint: cloudtrace.googleapis.com
      use_insecure: false
      attribute_mappings:
      - key: service.name
        replacement: g.co/gae/app/module
    metric:
      endpoint: monitoring.googleapis.com
      use_insecure: false
      prefix: workload.googleapis.com
      skip_create_descriptor: false
      known_domains:
      - mycustomdomain.googleapis.com
      instrumentation_library_labels: true
      service_resource_labels: true
      create_service_timeseries: false
      create_metric_descriptor_buffer_size: 10
      resource_filters:
      - prefix: k8s.*

    sending_queue:
      enabled: true
      num_consumers: 2
      queue_size: 50

    log:
      default_log_name: my-app
```

Beyond standard YAML configuration as outlined in the sections that follow,
exporters that leverage the net/http package (all do today) also respect the
following proxy environment variables:

* HTTP_PROXY
* HTTPS_PROXY
* NO_PROXY

If set at Collector start time then exporters, regardless of protocol,
will or will not proxy traffic as defined by these environment variables.

### Logging Exporter

The logging exporter processes OpenTelemetry log entries and exports them to GCP Cloud Logging. Logs can be collected using one 
of the opentelemetry-collector-contrib log receivers, such as the [filelogreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/filelogreceiver).

Log entries must contain any Cloud Logging-specific fields as a matching OpenTelemetry attribute (as shown in examples from the
[logs data model](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/logs/data-model.md#google-cloud-logging)).
These attributes can be parsed using the various [log operators](../../pkg/stanza/docs/operators/README.md#what-operators-are-available) available upstream.

For example, the following config parses the [HTTPRequest field](https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#HttpRequest) from Apache log entries saved in `/var/log/apache.log`. 
It also parses out the `timestamp` and inserts a non-default `log_name` attribute and GCP [MonitoredResource](https://cloud.google.com/logging/docs/reference/v2/rest/v2/MonitoredResource) attribute.

```yaml
receivers:
  filelog:
    include: [ /var/log/apache.log ]
    start_at: beginning
    operators:
      - id: http_request_parser
        type: regex_parser
        regex: '(?m)^(?P<remoteIp>[^ ]*) (?P<host>[^ ]*) (?P<user>[^ ]*) \[(?P<time>[^\]]*)\] "(?P<requestMethod>\S+)(?: +(?P<requestUrl>[^\"]*?)(?: +(?P<protocol>\S+))?)?" (?P<status>[^ ]*) (?P<responseSize>[^ ]*)(?: "(?P<referer>[^\"]*)" "(?P<userAgent>[^\"]*)")?$'
        parse_to: attributes["gcp.http_request"]
        timestamp:
          parse_from: attributes["gcp.http_request"].time
          layout_type: strptime
          layout: '%d/%b/%Y:%H:%M:%S %z'
    converter:
      max_flush_count: 100
      flush_interval: 100ms

exporters:
  googlecloud:
    project: my-gcp-project
    log:
      default_log_name: opentelemetry.io/collector-exported-log

processors:
  memory_limiter:
      check_interval: 1s
      limit_mib: 4000
      spike_limit_mib: 800
  resourcedetection:
    detectors: [gce, gke]
    timeout: 10s
  attributes:
    # Override the default log name.  `gcp.log_name` takes precedence
    # over the `default_log_name` specified in the exporter.
    actions:
      - key: gcp.log_name
        action: insert
        value: apache-access-log

service:
    logs:
      receivers: [filelog]
      processors: [memory_limiter, resourcedetection, attributes]
      exporters: [googlecloud]

```

This would parse logs of the following example structure:

```
127.0.0.1 - - [26/Apr/2022:22:53:36 +0800] "GET / HTTP/1.1" 200 1247
```

To the following GCP entry structure:

```
        {
          "logName": "projects/my-gcp-project/logs/apache-access-log",
          "resource": {
            "type": "gce_instance",
            "labels": {
              "instance_id": "",
              "zone": ""
            }
          },
          "textPayload": "127.0.0.1 - - [26/Apr/2022:22:53:36 +0800] \"GET / HTTP/1.1\" 200 1247",
          "timestamp": "2022-05-02T12:16:14.574548493Z",
          "httpRequest": {
            "requestMethod": "GET",
            "requestUrl": "/",
            "status": 200,
            "responseSize": "1247",
            "remoteIp": "127.0.0.1",
            "protocol": "HTTP/1.1"
          }
        }
```

The logging exporter also supports the full range of [GCP log severity levels](https://cloud.google.com/logging/docs/reference/v2/rpc/google.logging.type#google.logging.type.LogSeverity), 
which differ from the available [OpenTelemetry log severity levels](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/logs/data-model.md#severity-fields). 
To accommodate this, the following mapping is used to equate an incoming OpenTelemetry [`SeverityNumber`](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/logs/data-model.md#field-severitynumber) 
to a matching GCP log severity:

|OTel `SeverityNumber`/Name|GCP severity level|
|---|---|
|Undefined|Default|
|1-4 / Trace|Debug|
|5-8 / Debug|Debug|
|9-10 / Info|Info|
|11-12 / Info|Notice|
|13-16 / Warn|Warning|
|17-20 / Error|Error|
|21-22 / Fatal|Critical|
|23 / Fatal|Alert|
|24 / Fatal|Emergency|

The upstream [severity parser](../../pkg/stanza/docs/types/severity.md) (along
with the [regex parser](../../pkg/stanza/docs/operators/regex_parser.md)) allows for
additional flexibility in parsing log severity from incoming entries.

## Recommendations

It is recommended to always run a [batch processor](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/batchprocessor)
and [memory limiter](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/memorylimiterprocessor) for tracing pipelines to ensure
optimal network usage and avoiding memory overruns.  You may also want to run an additional
[sampler](../../processor/probabilisticsamplerprocessor), depending on your needs.

## Features and Feature-Gates

See the [Collector feature gates](https://github.com/open-telemetry/opentelemetry-collector/blob/main/service/featuregate/README.md#collector-feature-gates) for an overview of feature gates in the collector.

**BETA**: `exporter.googlecloud.OTLPDirect`

The `exporter.googlecloud.OTLPDirect` is enabled by default starting in v0.50.0, and can be disabled via `--feature-gates=-exporter.googlecloud.OTLPDirect`. The new googlecloud exporter translates pdata directly to google cloud monitoring's types, rather than first translating to opencensus.  See the [Breaking Changes documentation](https://github.com/GoogleCloudPlatform/opentelemetry-operations-go/blob/main/exporter/collector/breaking-changes.md#breaking-changes-vs-old-googlecloud-exporter) for breaking changes that will occur as a result of this feature.

If you are broken by changes described there, or have encountered an issue with the new implementation, please open an issue [here](https://github.com/GoogleCloudPlatform/opentelemetry-operations-go/issues/new)

If you disable the feature-gate, you can continue to set the legacy configuration options:

- `project` (optional): GCP project identifier.
- `endpoint` (optional): Endpoint where data is going to be sent to.
- `user_agent` (optional): Override the user agent string sent on requests to Cloud Monitoring (currently only applies to metrics). Specify `{{version}}` to include the application version number. Defaults to `opentelemetry-collector-contrib {{version}}`.
- `use_insecure` (optional): If true. use gRPC as their communication transport. Only has effect if Endpoint is not "".
- `timeout` (optional): Timeout for all API calls. If not set, defaults to 12 seconds.
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

[beta]:https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib