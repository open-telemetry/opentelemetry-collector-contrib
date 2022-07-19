# Datadog Exporter

| Status                   |                  |
| ------------------------ |------------------|
| Stability                | [beta]           |
| Supported pipeline types | traces, metrics  |
| Distributions            | [contrib], [AWS] |

This exporter sends metric and trace data to [Datadog](https://datadoghq.com). For environment specific setup instructions visit the [Datadog Documentation](https://docs.datadoghq.com/tracing/setup_overview/open_standards/otel_collector_datadog_exporter/).

> Please review the Collector's [security
> documentation](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/security.md),
> which contains recommendations on securing sensitive information such as the
> API key required by this exporter.

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

The hostname can be set in the configuration or via semantic conventions. If none is present, the exporter will add one based on the environment.

See the sample configuration files under the `example` folder for other available options, as well as an example K8s Manifest.
This exporter also supports the `exporterhelper` queuing, retry and timeout settings documented [here](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/exporterhelper#configuration).
Retry settings will only affect metrics.

## Support for Span Events

*Please Note:* Currently [Span Events](https://github.com/open-telemetry/opentelemetry-specification/blob/11cc73939a32e3a2e6f11bdeab843c61cf8594e9/specification/trace/api.md#add-events) are extracted and added to Spans as Json on the Datadog Span Tag `events`.

[beta]:https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
[AWS]:https://aws-otel.github.io/docs/partners/datadog
