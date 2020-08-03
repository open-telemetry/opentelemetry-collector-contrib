# Datadog Exporter

This exporter sends metric data to [Datadog](https://datadoghq.com).

## Configuration

The only required setting is a [Datadog API key](https://app.datadoghq.com/account/settings#api).
To send your Agent data to the Datadog EU site, set the site parameter to:
``` yaml
site: datadoghq.eu
```

See the sample configuration file under the `example` folder for other available options.
