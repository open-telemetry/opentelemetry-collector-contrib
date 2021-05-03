# New Relic Exporter

:sparkles: **[New Relic offers OTLP (OpenTelemetry Protocol) Ingest as a pre-release! To sign up, click here!](https://forms.gle/fa2pWcQxgVQYMggEA)** :sparkles:

This means you can use an [OTLP](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/otlpexporter) exporter and no longer need this exporter to send data to New Relic.

## Overview

This exporter supports sending trace, metric, and log data to [New Relic](https://newrelic.com/)

> Please review the Collector's [security
> documentation](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/security.md),
> which contains recommendations on securing sensitive information such as the
> API key required by this exporter.

## Configuration

The following common configuration options are supported:

* One or both of the following are required:
  * `apikey`: Your New Relic [Insights Insert API Key](https://docs.newrelic.com/docs/insights/insights-data-sources/custom-data/send-custom-events-event-api#register).
  * `api_key_header`: Request header to read New Relic [Insights Insert API Key](https://docs.newrelic.com/docs/insights/insights-data-sources/custom-data/send-custom-events-event-api#register) from.
* `timeout` (Optional): Amount of time spent attempting a request before abandoning and dropping data. Default is 15 seconds.
* `host_override` (Optional): Overrides the host to which data is sent. The URL will be generated in the form:
  https://\$host/\$path. Only set the the host portion of the URL. The path component **CANNOT** be overridden.

**Basic example:**
```yaml
exporters:
  newrelic:
    apikey: super-secret-api-key
    timeout: 30s
```

Configuration option can be overridden by telemetry signal (i.e., traces,
metrics, and logs). This is especially important if you need to use the
`host_override` option because the exporter defaults to sending data to New
Relic's US data centers. For other use cases refer to
[OpenTelemetry: Advanced configuration](https://docs.newrelic.com/docs/integrations/open-source-telemetry-integrations/opentelemetry/opentelemetry-advanced-configuration#h2-change-endpoints).

**Example of overriding options by telemetry signal:**
```yaml
exporters:
  newrelic:
    apikey: super-secret-api-key
    timeout: 30s

    # host_override is set to send data to New Relic's EU data centers.
    traces:
      host_override: trace-api.eu.newrelic.com
      timeout: 20s
    metrics:
      host_override: metric-api.eu.newrelic.com
    logs:
      host_override: log-api.eu.newrelic.com
```

## Find and use your data

Once the exporter is sending data you can start to explore your data in New Relic:

- Metric data: see [Metric API docs](https://docs.newrelic.com/docs/data-ingest-apis/get-data-new-relic/metric-api/introduction-metric-api#find-data).
- Trace/span data: see [Trace API docs](https://docs.newrelic.com/docs/understand-dependencies/distributed-tracing/trace-api/introduction-trace-api#view-data).
- Log data: see [Log docs](https://docs.newrelic.com/docs/logs/log-management/ui-data/explore-your-data-log-analytics)

For general querying information, see:

- [Query New Relic data](https://docs.newrelic.com/docs/using-new-relic/data/understand-data/query-new-relic-data)
- [Intro to NRQL](https://docs.newrelic.com/docs/query-data/nrql-new-relic-query-language/getting-started/nrql-syntax-clauses-functions)
