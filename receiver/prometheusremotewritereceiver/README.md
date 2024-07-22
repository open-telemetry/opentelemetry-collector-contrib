# Prometheus Remote Write Receiver

The `prometheus_remote_write_receiver` receiver exposes a compatible
Prometheus remoteWrite endpoint.  Prometheus remoteWrite protocol
may not specify the type of metric being produced.  This receiver
offers a configuration mapping based upon metric naming that will
create the correct Otel metric type.

See [Prometheus Proto Types](https://github.com/prometheus/prometheus/blob/main/prompb/)

## Limited Metric Types and Mapping

### Supported

The mappings of suffixes to Otel Metric types is limited to
the counter and gauge types.

### Not Supported

The MetricMetadata is not considered in the current version.
Mappings to `histogram` and `summary` are not implemented.

## Configuration

This receiver was modeled after the [otlpreciever](https://github.com/open-telemetry/opentelemetry-collector/blob/main/receiver/otlpreceiver).  It supports the standard configuration available in
this receiver.  The Prometheus remoteWrite protocol is HTTP, but this
receiver also supports gRPC.

The following settings are required:

- `protocols`: Specifies the listening ports and protocols
- `metric_types`: provides suffix mapping naming conventions
  - `counter`: metric name suffix that identifies sum metrics

Example:

```yaml
receivers:
  prometheusremotewritereceiver:
    protocols:
      grpc:
        endpoint: localhost:4317
      http:
        endpoint: localhost:4318
    metric_types:
      counter:
        - _counter
        - _sum
        - _total
        - _count
        - _bytes_ingress
        - _bytes_egress
      gauge:
        - _gauge
      histogram:
      summary:

processors:
  batch:

exporters:
  debug:
    verbosity: 'detailed'

service:
  pipelines:
    metrics:
      receivers:
        - prometheusremotewritereceiver
      processors:
        - batch
      exporters:
        - debug

```

The full list of settings exposed for this receiver are documented [here](./config.go)
with detailed sample configurations [here](./testdata/config.yaml).
