# Datadog Exporter

This exporter sends metric and trace data to [Datadog](https://datadoghq.com). For environment specific setup instructions visit the [Datadog Documentation](https://docs.datadoghq.com/tracing/setup_overview/open_standards/#opentelemetry-collector-datadog-exporter).

> Please review the Collector's [security
> documentation](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/security.md),
> which contains recommendations on securing sensitive information such as the
> API key required by this exporter.

> **Note**: The metrics exporter is not considered stable yet and may suffer breaking changes.

## Configuration

The only required setting is a [Datadog API key](https://app.datadoghq.com/account/settings#api).

```yaml
datadog:
  api:
    key: "<API key>"
```
 
 To send data to the Datadog EU site, set the `api.site` parameter to `datadoghq.eu`:

```yaml
datadog:
  api:
    key: "<API key>"
    site: datadoghq.eu
```

The hostname, environment, service and version can be set in the configuration for unified service tagging.
The exporter will try to retrieve a hostname following the OpenTelemetry semantic conventions if there is one available.

See the sample configuration files under the `example` folder for other available options, as well as an example K8s Manifest.

## Trace exporter
### **Important Pipeline Setup Details** 

This exporter assumes a pipeline using the datadog exporter also includes a [batch processor](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/batchprocessor) configured with the following: 
  - a `timeout` setting of `10s`(10 seconds). 

Please make sure to include this processor in your pipeline. An example pipeline can be found below.

A batch representing 10 seconds of traces is a constraint of Datadog's API Intake for Trace Related Statistics. Without this setting, trace related metrics including `.hits` `.errors` and `.duration` for different services and service resources may be inaccurate over periods of time.

Example:

```yaml
receivers:
  examplereceiver:

processors:
  batch:
    timeout: 10s

exporters:
  datadog/api:
    hostname: customhostname
    env: prod
    service: myservice
    version: myversion

    tags:
      - example:tag

    api:
      key: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
      site: datadoghq.eu

service:
  pipelines:
    traces:
      receivers: [examplereceiver]
      processors: [batch]
      exporters: [datadog/api]
```

### Span Events

*Please Note:* Currently [Span Events](https://github.com/open-telemetry/opentelemetry-specification/blob/11cc73939a32e3a2e6f11bdeab843c61cf8594e9/specification/trace/api.md#add-events) are extracted and added to Spans as Json on the Datadog Span Tag `events`.

### Recommended Samplers

While the OpenTelemetry Specification for Sampling [remains undecided and in active development](https://github.com/open-telemetry/oteps/pull/148), the Datadog Exporter currently supports two sampling approaches to ensure accuracy in generated Trace Stats payloads for details such as hits, errors, and latency within Datadog.

1. [`ParentBased(root=AlwaysOn)`](https://github.com/open-telemetry/opentelemetry-specification/blob/7fc28733eb3791ebcc98fed0d858a7961f1e95b2/specification/trace/sdk.md#alwayson):  This is the default SDK sampler, and emits 100% of Spans. Depending on your use case, in high volume environments this approach may result in higher than expected resource consumption.

2. [`DatadogTraceIdRatioBased` , a customized `TraceIdRatioBased` sampler used as the root of a `ParentBased` Sampler, ex: `ParentBased(root=DatadogTraceIdRatioBased)`](https://github.com/open-telemetry/opentelemetry-specification/blob/7fc28733eb3791ebcc98fed0d858a7961f1e95b2/specification/trace/sdk.md#traceidratiobased): In certain high volume environments, an `AlwaysOn` Sampler may not be feasible. In these cases, Datadog supports a global, head based, probability sampling approach for OpenTelemetry by including a custom Datadog `TraceIdRatioBased` Sampler used as the root in a `ParentBased` composite Sampler. This sampler ensures that the global `probability` sampling rate information is appended to Sampled Spans as an attribute `_sample_rate`. This attribute has semantic significance within Datadog and ensures that the Datadog Exporter can correctly "upscale" generated Trace Stats payloads to ensure accurate hits, errors, and latency statistics that include dropped spans. For an example of this custom sampler, [see the example application in [/examples/example_custom_sampler](/examples/example_custom_sampler/example_custom_sampler.go.example).

## Metric exporter

The metrics exporter does not assume any specific pipeline setup.
There are a number of optional settings for configuring how to send your metrics:

| Option name | Description | Default |
|-|-|-|
| `send_monotonic_counter` | Cumulative monotonic metrics are sent as deltas between successive measurements. Disable this flag to send get the raw, monotonically increasing value. | `true` |
| `delta_ttl` | Maximum number of seconds values from cumulative monotonic metrics are kept in memory. | 3600 |
| `report_quantiles` | Whether to report quantile values for summary type metrics. | `true` |
