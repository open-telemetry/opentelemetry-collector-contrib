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
    - `application` is set to "defaultApp". You can override the default value using the exporter configuration.
    - `service` is set to "defaultService". You can override the default value using the exporter configuration.

## Exporter Configuration

```yaml
receivers:
  examplereceiver:

processors:
  batch:
    timeout: 10s

exporters:
  tanzuobservability:
    traces:
      # Hostname and `customTracingListenerPorts` of the Wavefront Proxy
      endpoint: "http://localhost:30001"
      # Override the default of 'application=defaultApp' for spans missing the tag
      default_application: "my-application"
      # Override the default of 'service=defaultService' for spans missing the tag
      default_service: "queue-service"

service:
  pipelines:
    traces:
      receivers: [examplereceiver]
      processors: [batch]
      exporters: [tanzuobservability]
```
