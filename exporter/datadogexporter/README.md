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
