# Stackdriver Exporter

This exporter can be used to send metrics and traces to Google Cloud Monitoring and Trace (formerly known as Stackdriver) respectively.

The following configuration options are supported:

- `project` (optional): GCP project identifier.
- `endpoint` (optional): Endpoint where data is going to be sent to.
- `user_agent` (optional): Override the user agent string sent on requests to Cloud Monitoring (currently only applies to metrics). Specify `{{version}}` to include the application version number. Defaults to `opentelemetry-collector-contrib {{version}}`.
- `use_insecure` (optional): If true. use gRPC as their communication transport. Only has effect if Endpoint is not "".
- `timeout` (optional): Timeout for all API calls. If not set, defaults to 12 seconds.
- `number_of_workers` (optional): NumberOfWorkers sets the number of go rountines that send requests. The minimum number of workers is 1.
- `resource_mappings` (optional): ResourceMapping defines mapping of resources from source (OpenCensus) to target (Stackdriver).
  - `label_mappings` (optional): Optional flag signals whether we can proceed with transformation if a label is missing in the resource.

Additional configuration for the trace exporter:

- `trace.bundle_delay_threshold` (optional): Starting from the time that the first span is added to a bundle, once this delay has passed, handle the bundle. If not set, uses the exporter default.
- `trace.bundle_count_threshold` (optional): Once a bundle has this many spans, handle the bundle. Since only one span at a time is added to a bundle, no bundle will exceed this threshold, so it also serves as a limit. If not set, uses the exporter default.
- `trace.bundle_byte_threshold` (optional): Once the number of bytes in current bundle reaches this threshold, handle the bundle. This triggers handling, but does not cap the total size of a bundle. If not set, uses the exporter default.
- `trace.bundle_byte_limit` (optional): The maximum size of a bundle, in bytes. Zero means unlimited.
- `trace.buffer_max_bytes` (optional): The maximum number of bytes that the Bundler will keep in memory before returning ErrOverflow. If not set, uses the exporter default.

Additional configuration for the metric exporter:

- `metric.prefix` (optional): MetricPrefix overrides the prefix / namespace of the Stackdriver metric type identifier. If not set, defaults to "custom.googleapis.com/opencensus/"
- `metric.skip_create_descriptor` (optional): Whether to skip creating the metric descriptor.

Example:

```yaml
exporters:
  stackdriver:
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
          - source_key: contrib.opencensus.io/exporter/stackdriver/project_id
            target_key: project_id
            optional: true
          - source_key: source.label1
            target_key: target_label_1

    trace:
      bundle_delay_threshold: 2s
      bundle_count_threshold: 50
      bundle_byte_threshold: 15e3
      bundle_byte_limit: 0
      buffer_max_bytes: 8e6

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
