# Datadog Processor

| Status                   |               |
|--------------------------|---------------|
| Stability                | [development] |
| Supported pipeline types | traces        |
| Distributions            | [contrib]     |

## Description

The Datadog Processor can be used to compute Datadog APM Stats pre-sampling. For example, when using the [tailsamplingprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/tailsamplingprocessor#tail-sampling-processor) or [probabilisticsamplerprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/probabilisticsamplerprocessor) components, the `datadogprocessor` can be prepended into the pipeline to ensure that Datadog APM Stats are accurate and include the dropped traces.

## Configuration

By default, when used in conjunction with the Datadog Exporter, the processor should detect its presence (as long as it is configured within a pipeline), and use it to export the Datadog APM Stats.

If using within a gateway deployment or alongside the Datadog Agent, where the Datadog Exporter is not present, and, for example, an OTLP exporter might be, you need to specify the metrics exporter to the processor:

```yaml
processors:
  datadog:
    metrics_exporter: otlp
```

The default value for `metrics_exporter` is `datadog`. Any configured metrics exporter must exist as part of a metrics pipeline.

When using in conjunction with the Datadog Agent's OTLP Ingest, the minimum required Datadog Agent version that supports this processor is 7.42.0.

If not using the Datadog backend, the processor will still create valid RED metrics, but in that situation you may prefer to use the [spanmetricsprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/spanmetricsprocessor) instead.

[development]: https://github.com/open-telemetry/opentelemetry-collector#development
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
