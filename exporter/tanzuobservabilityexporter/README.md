# Tanzu Observability (Wavefront) Exporter

This exporter supports sending traces to [Tanzu Observability](https://tanzu.vmware.com/observability).

## Prerequisites

- [Obtain the Tanzu Observability by Wavefront API token.](https://docs.wavefront.com/wavefront_api.html#generating-an-api-token)
- [Set up and start a Tanzu Observability by Wavefront proxy](https://docs.wavefront.com/proxies_installing.html) and
  configure it with the API token you obtained.
- To have the proxy generate [span RED metrics](https://docs.wavefront.com/trace_data_details.html#red-metrics) from
  trace data, [configure](https://docs.wavefront.com/proxies_configuring.html) the proxy's `customTracingListenerPorts`
  and use it for the exporter's endpoint.

## Data Conversion

- Trace IDs and Span IDs are converted to UUIDs. For example, span IDs are left-padded with zeros to fit the correct
  size.
- Events are converted to [Span Logs](https://docs.wavefront.com/trace_data_details.html#span-logs).
- Kind is converted to the `span.kind` tag.
- Status is converted to `error`, `status.code` and `status.message` tags.
- TraceState is converted to the `w3c.tracestate` tag.

## Tanzu Observability Specific Attributes

- Application identity tags, which
  are [required by Tanzu Observability](https://docs.wavefront.com/trace_data_details.html#how-wavefront-uses-application-tags), are added if they are missing.
    - `application` is set to "defaultApp".
    - `service` is set to "defaultService".

## Configuration

The only required configuration is a Wavefront proxy API endpoint to receive traces from the Tanzu Observability
Exporter.

Given a Wavefront proxy at `10.10.10.10`, configured with `customTracingListenerPorts` set to `30001`, the traces
endpoint would be `http://10.10.10.10:30001`.

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
      receivers: [ examplereceiver ]
      processors: [ batch ]
      exporters: [ tanzuobservability ]
```

### Advanced Configuration

#### Processor

The memory limiter processor is used to prevent out of memory situations on the collector. It allows performing periodic
checks of memory usage if it exceeds defined limits will begin dropping data and forcing GC to reduce memory
consumption.

* `memory_limiter` [Details and defaults here](https://github.com/open-telemetry/opentelemetry-collector/blob/main/processor/memorylimiterprocessor/README.md)
    * `check_interval`
        * 0s by default so, no checks will be performed.
        * It is the time between measurements of memory usage. The recommended value is `1s`. Values below `1s` are not recommended since it can result in unnecessary CPU consumption.
    * `limit_mib`
        * 0 by default so, the hard limit will be set as 0MiB, which will cause the processor to forcefully perform
          garbage collection.
        * Maximum amount of memory, in MiB, targeted to be allocated by the process heap.
    * `spike_limit_mib`
        * 0 by default so, the soft limit will be set as 0Mib, which will cause processor to drop data and return
          errors.
        * Maximum spike expected between the measurements of memory usage. The recommended value is 20% of `limit_mib`.
    * `limit_percentage`
        * 0 by default so, the hard limit will be set as 0MiB, which will cause the processor to forcefully perform
          garbage collection.
        * This config is used to calculate `limit_mib` from the total available memory. Percentage configuration is
          recommended over the fixed memory config `limit_mib`. More over this configuration is supported on Linux
          systems with cgroups and it's intended to be used in dynamic platforms like docker. However, `limit_mib` takes
          precedence over the percentage configuration.
    * `spike_limit_percentage`
        * 0 by default so, the soft limit will be set as 0Mib, which will cause processor to drop data and return
          errors.
        * This option is used to calculate `spike_limit_mib` from the total available memory. This option is intended to
          be used only with `limit_percentage`.

#### Exporter

This exporter
uses [queuing and retry helpers](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/exporterhelper/README.md)
provided by the core OpenTelemetry Collector. The `retry_on_failure` and `sending_queue` features are enabled by
default, but can be disabled using the options below.

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
  memory_limiter:
    check_interval: 1s
    limit_percentage: 50
    spike_limit_percentage: 30

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
      receivers: [ examplereceiver ]
      processors: [ batch ]
      exporters: [ tanzuobservability ]
```
