# Datadog Exporter

This exporter sends metric data to [Datadog](https://datadoghq.com).

## Configuration

The metrics exporter has two modes:
  - If sending metrics through DogStatsD (the default mode), there are no required settings. 
  - If sending metrics without an Agent the mode must be set explicitly and
      you must provide a [Datadog API key](https://app.datadoghq.com/account/settings#api).
 ```yaml
datadog:
  api:
    key: "<API key>"
    # site: datadoghq.eu for sending data to the Datadog EU site
   metrics:
     mode: agentless
 ```

The hostname, environment, service and version can be set in the configuration for unified service tagging.

See the sample configuration file under the `example` folder for other available options.
