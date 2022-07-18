# Datadog Exporter

| Status                   |                 |
| ------------------------ |-----------------|
| Stability                | [beta]          |
| Supported pipeline types | traces, metrics |
| Distributions            | [contrib]       |

This exporter sends metric and trace data to [Datadog](https://datadoghq.com). For environment specific setup instructions visit the [Datadog Documentation](https://docs.datadoghq.com/tracing/setup_overview/open_standards/otel_collector_datadog_exporter/).

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

 If you want to use the OpenTelemetry Span Name as the Datadog Resource Name you can set the `span_name_as_resource_name` configuration option to `true` (default is `false`). For more info on the downsides of this option check [this](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/1909) issue.

 ```yaml
datadog:
  api:
    key: "<API key>"
  traces:
    span_name_as_resource_name: true
```

The hostname, environment, service and version can be set in the configuration for unified service tagging.
The exporter will try to retrieve a hostname following the OpenTelemetry semantic conventions if there is one available.

See the sample configuration files under the `example` folder for other available options, as well as an example K8s Manifest.
This exporter also supports the `exporterhelper` queuing, retry and timeout settings documented [here](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/exporterhelper#configuration).
Retry settings will only affect metrics.

## Support for Span Events

*Please Note:* Currently [Span Events](https://github.com/open-telemetry/opentelemetry-specification/blob/11cc73939a32e3a2e6f11bdeab843c61cf8594e9/specification/trace/api.md#add-events) are extracted and added to Spans as Json on the Datadog Span Tag `events`.

## Recommended Samplers

While the OpenTelemetry Specification for Sampling [remains undecided and in active development](https://github.com/open-telemetry/oteps/pull/148), the Datadog Exporter currently supports two sampling approaches to ensure accuracy in generated Trace Stats payloads for details such as hits, errors, and latency within Datadog.

1. [`ParentBased(root=AlwaysOn)`](https://github.com/open-telemetry/opentelemetry-specification/blob/7fc28733eb3791ebcc98fed0d858a7961f1e95b2/specification/trace/sdk.md#alwayson):  This is the default SDK sampler, and emits 100% of Spans. Depending on your use case, in high volume environments this approach may result in higher than expected resource consumption.

2. [`DatadogTraceIdRatioBased`, a customized `TraceIdRatioBased` sampler used as the root of a `ParentBased` Sampler, ex: `ParentBased(root=DatadogTraceIdRatioBased)`](https://github.com/open-telemetry/opentelemetry-specification/blob/7fc28733eb3791ebcc98fed0d858a7961f1e95b2/specification/trace/sdk.md#traceidratiobased): In certain high volume environments, an `AlwaysOn` Sampler may not be feasible. In these cases, Datadog supports a global, head based, probability sampling approach for OpenTelemetry by including a custom Datadog `TraceIdRatioBased` Sampler used as the root in a `ParentBased` composite Sampler. This sampler ensures that the global `probability` sampling rate information is appended to Sampled Spans as an attribute `_sample_rate`. This attribute has semantic significance within Datadog and ensures that the Datadog Exporter can correctly "upscale" generated Trace Stats payloads to ensure accurate hits, errors, and latency statistics that include dropped spans. For an example of this custom sampler, see the example application in [example/example_custom_sampler](example/example_custom_sampler/example_custom_sampler.go.example).

[beta]:https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
