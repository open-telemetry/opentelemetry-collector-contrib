# Datadog Exporter

This exporter sends metric and trace data to [Datadog](https://datadoghq.com). For environment specific setup instructions visit the [Datadog Documentation](https://docs.datadoghq.com/tracing/setup_overview/open_standards/#opentelemetry-collector-datadog-exporter).

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

## Metric exporter

The metrics exporter does not assume any specific pipeline setup.
There are a number of optional settings for configuring how to send your metrics:

| Option name | Description | Default |
|-|-|-|
| `send_monotonic_counters` | Cumulative monotonic metrics are sent as deltas between successive measurements. Disable this flag to send get the raw, monotonically increasing value. | `true` |
| `delta_ttl` | Maximum number of seconds values from cumulative monotonic metrics are kept in memory. | 3600 |
