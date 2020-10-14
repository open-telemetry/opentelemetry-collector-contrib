# Datadog Exporter

This exporter sends metric and trace data to [Datadog](https://datadoghq.com).

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

See the sample configuration file under the `example` folder for other available options.

*IMPORTANT FOR TRACE EXPORT*: This exporter assumes that a pipeline which includes the datadog exporter also includes a [batch processor](https://github.com/open-telemetry/opentelemetry-collector/tree/master/processor/batchprocessor) configured with a `timeout` setting of `10s`(10 seconds). A batch representing 10 seconds of traces is a constraint of Datadog's API Intake for Trace Related Statistics. Without this setting, trace related metrics including `.hits` `.errors` and `.duration` for different services may be inaccurate. An example suggested configuration for this processor is included below:

Examples:

```
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
