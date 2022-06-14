# Google Managed Service for Prometheus Exporter

| Status                   |                       |
| ------------------------ |-----------------------|
| Stability                | [in development](https://github.com/open-telemetry/opentelemetry-collector#in-development) |
| Supported pipeline types | metrics               |
| Distributions            | []                    |

This exporter can be used to send metrics and traces to [Google Cloud Managed Service for Prometheus](https://cloud.google.com/stackdriver/docs/managed-prometheus).  The difference between this exporter and the `googlecloud` exporter is that metrics sent with this exporter are queried using [promql](https://prometheus.io/docs/prometheus/latest/querying/basics/#querying-prometheus), rather than standard the standard MQL.

This exporter is not the standard method of ingesting metrics into Google Cloud Managed Service for Prometheus, which is built on a drop-in replacement for the Prometheus server: https://github.com/GoogleCloudPlatform/prometheus.  This exporter does not support the full range of Prometheus functionality, including the UI, recording and alerting rules, and can't be used with the GMP Operator, but does support sending metrics.

## Example Configuration

```yaml
receivers:
    prometheus:
        config:
          scrape_configs:
            # Add your prometheus scrape configuration here.
            # Using kubernetes_sd_configs with namespaced resources
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


