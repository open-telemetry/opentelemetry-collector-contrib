# Google Managed Service for Prometheus Exporter

| Status                   |                       |
| ------------------------ |-----------------------|
| Stability                | [alpha](https://github.com/open-telemetry/opentelemetry-collector#alpha) |
| Supported pipeline types | metrics               |
| Distributions            | [contrib](https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib)    |

This exporter can be used to send metrics and traces to [Google Cloud Managed Service for Prometheus](https://cloud.google.com/stackdriver/docs/managed-prometheus).  The difference between this exporter and the `googlecloud` exporter is that metrics sent with this exporter are queried using [promql](https://prometheus.io/docs/prometheus/latest/querying/basics/#querying-prometheus), rather than standard the standard MQL.

This exporter is not the standard method of ingesting metrics into Google Cloud Managed Service for Prometheus, which is built on a drop-in replacement for the Prometheus server: https://github.com/GoogleCloudPlatform/prometheus.  This exporter does not support the full range of Prometheus functionality, including the UI, recording and alerting rules, and can't be used with the GMP Operator, but does support sending metrics.

## Configuration Reference

The following configuration options are supported:

- `project` (optional): GCP project identifier.
- `user_agent` (optional): Override the user agent string sent on requests to Cloud Monitoring (currently only applies to metrics). Specify `{{version}}` to include the application version number. Defaults to `opentelemetry-collector-contrib {{version}}`.
- `endpoint` (optional): Endpoint where metric data is going to be sent to. Replaces `endpoint`.
- `use_insecure` (optional): If true, use gRPC as their communication transport. Only has effect if Endpoint is not "".
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

## Example Configuration

```yaml
receivers:
    prometheus:
        config:
          scrape_configs:
            # Add your prometheus scrape configuration here.
            # Using kubernetes_sd_configs with namespaced resources (e.g. pod)
            # ensures the namespace is set on your metrics.
            - job_name: 'kubernetes-pods'
                kubernetes_sd_configs:
                - role: pod
                relabel_configs:
                - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
                action: keep
                regex: true
                - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
                action: replace
                target_label: __metrics_path__
                regex: (.+)
                - source_labels: [__address__, __meta_kubernetes_pod_annotation_prometheus_io_port]
                action: replace
                regex: (.+):(?:\d+);(\d+)
                replacement: $$1:$$2
                target_label: __address__
                - action: labelmap
                regex: __meta_kubernetes_pod_label_(.+)
processors:
    # groupbyattrs promotes labels from metrics to resources, allowing them to
    # be added to the prometheus_target monitored resource.
    # This allows exporters which monitor multiple namespaces, such as
    # kube-state-metrics, to override the namespace in the resource by setting
    # metric labels.
    groupbyattrs:
      keys:
        - namespace
        - cluster
        - location
    batch:
        # batch metrics before sending to reduce API usage
        send_batch_max_size: 200
        send_batch_size: 200
        timeout: 5s
    memory_limiter:
        # drop metrics if memory usage gets too high
        check_interval: 1s
        limit_percentage: 65
        spike_limit_percentage: 20
    resourcedetection:
        # detect cluster name and location
        detectors: [gcp]
        timeout: 10s
exporters:
    googlemanagedprometheus:

service:
  pipelines:
    metrics:
      receivers: [prometheus]
      processors: [groupbyattrs, batch, memory_limiter, resourcedetection]
      exporters: [googlemanagedprometheus]
```

## Resource Attribute Handling

The Google Managed Prometheus exporter maps metrics to the
[prometheus_target](https://cloud.google.com/monitoring/api/resources#tag_prometheus_target)
monitored resource. The logic for mapping to monitored resources is designed to
be used with the prometheus receiver, but can be used with other receivers as
well. To avoid collisions (i.e. "duplicate timeseries enountered" errors), you
need to ensure the prometheus_target resource uniquely identifies the source of
metrics. The exporter uses the following resource attributes to determine
monitored resource:

* location: [`location` (see `groupbyattrs` config above), `cloud.availability_zone`, `cloud.region`]
* cluster: [`cluster` (see `groupbyattrs` config above), `k8s.cluster.name`]
* namespace: [`namespace` (see `groupbyattrs` config above), `k8s.namespace.name`]
* job: [`service.name` + `service.namespace`]
* instance: [`service.instance.id`]

In the configuration above, `cloud.availability_zone`, `cloud.region`, and
`k8s.cluster.name` are detected using the `resourcedetection` processor with
the `gcp` detector. The prometheus receiver sets `service.name` to the
configured `job_name`, and `service.instance.id` is set to the scrape target's
`instance`. The prometheus receiver sets `k8s.namespace.name` when using
`role: pod`.  The `groupbyattrs` processor promotes `location`, `cluster`, and
`namespace` labels on metrics to resource labels, which allows overriding (e.g.
using prometheus metric_relabel_configs) the attributes discovered by other
receivers or processors. This is useful when metric exporters already have
`location`, `cluster`, or `namespace` labels, such as Kube-state-metrics.
