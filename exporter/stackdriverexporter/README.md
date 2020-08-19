# Stackdriver Exporter

This exporter can be used to send metrics and traces to Google Cloud Monitoring and Trace (formerly known as Stackdriver) respectively.

The following configuration options are supported:

- `project` (optional): GCP project identifier.
- `endpoint` (optional): Endpoint where data is going to be sent to.
- `metric_prefix` (optional): MetricPrefix overrides the prefix of a Stackdriver metric names.
- `number_of_workers` (optional): NumberOfWorkers sets the number of go rountines that send requests. The minimum number of workers is 1.
- `use_insecure` (optional): If true. use gRPC as their communication transport. Only has effect if Endpoint is not "".
- `skip_create_metric_descriptor` (optional): Whether to skip creating the metric descriptor.
- `resource_mappings` (optional): ResourceMapping defines mapping of resources from source (OpenCensus) to target (Stackdriver).
- `label_mappings`.`optional` (optional): Optional flag signals whether we can proceed with transformation if a label is missing in the resource.
Example:

```yaml
exporters:
  stackdriver:
  stackdriver/customname:
    project: my-project
    metric_prefix: prefix
    endpoint: test-endpoint
    number_of_workers: 3
    skip_create_metric_descriptor: true
    use_insecure: true
    resource_mappings:
      - source_type: source.resource1
        target_type: target-resource1
        label_mappings:
          - source_key: contrib.opencensus.io/exporter/stackdriver/project_id
            target_key: project_id
            optional: true
          - source_key: source.label1
            target_key: target_label_1
```

Beyond standard YAML configuration as outlined in the sections that follow,
exporters that leverage the net/http package (all do today) also respect the
following proxy environment variables:

* HTTP_PROXY
* HTTPS_PROXY
* NO_PROXY

If set at Collector start time then exporters, regardless of protocol,
will or will not proxy traffic as defined by these environment variables.
