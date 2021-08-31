# Tanzu Observability (Wavefront) Exporter

This exporter supports sending traces to [Tanzu Observability](https://tanzu.vmware.com/observability).

## Prerequisites

- [Obtain the Tanzu Observability by Wavefront API token.](https://docs.wavefront.com/wavefront_api.html#generating-an-api-token)
- [Set up and start a Tanzu Observability by Wavefront proxy](https://docs.wavefront.com/proxies_installing.html) and configure it with the API token you obtained.
- To have the proxy generate [span RED metrics](https://docs.wavefront.com/trace_data_details.html#red-metrics) from trace data, [configure](https://docs.wavefront.com/proxies_configuring.html) the proxy's `customTracingListenerPorts` and use it for the exporter's endpoint.

## Data Conversion

- Trace IDs and Span IDs are converted to UUIDs. For example, span IDs are left-padded with zeros to fit the correct size.
- Events are converted to [Span Logs](https://docs.wavefront.com/trace_data_details.html#span-logs).
- Kind is converted to the `span.kind` tag.
- Status is converted to `error`, `status.code` and `status.message` tags.
- TraceState is converted to the `w3c.tracestate` tag.

## Tanzu Observability Specific Attributes

- Application identity tags, which are [required by Tanzu Observability](https://docs.wavefront.com/trace_data_details.html#how-wavefront-uses-application-tags), are added if they are missing.
    - `application` is set to "defaultApp".
    - `service` is set to "defaultService".

## Configuration

The only required configuration is a Wavefront proxy API endpoint to receive traces from the Tanzu Observability Exporter.

Given a Wavefront proxy at `10.10.10.10`, configured with `customTracingListenerPorts` set to `30001`, the traces endpoint would be `http://10.10.10.10:30001`.

### Example Configuration

```yaml
receivers:
  examplereceiver:

processors:
  batch:
    timeout: 10s

exporters:
  tanzuobservability:
    traces:
      endpoint: "http://10.10.10.10:30001"

service:
  pipelines:
    traces:
      receivers: [examplereceiver]
      processors: [batch]
      exporters: [tanzuobservability]
```

### Advanced Configuration

This exporter uses [queuing and retry helpers](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/exporterhelper/README.md) provided by the core OpenTelemetry Collector. The `retry_on_failure` and `sending_queue` features are enabled by default, but can be disabled using the options below.

* `retry_on_failure` [Details and defaults here](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/exporterhelper/README.md#configuration). Enabled by default.
    * `enabled`
    * `initial_interval`
    * `max_interval`
    * `max_elapsed_time`
* `sending_queue` [Details and defaults here](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/exporterhelper/README.md#configuration). Enabled by default.
    * `enabled`
    * `num_consumers`
    * `queue_size`

```yaml
receivers:
  examplereceiver:

processors:
  batch:
    timeout: 10s

exporters:
  tanzuobservability:
    traces:
      endpoint: "http://10.10.10.10:30001"
    retry_on_failure:
      max_elapsed_time: 3m
    sending_queue:
      queue_size: 10000

service:
  pipelines:
    traces:
      receivers: [examplereceiver]
      processors: [batch]
      exporters: [tanzuobservability]
```
