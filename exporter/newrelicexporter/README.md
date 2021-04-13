# New Relic Exporter

This exporter supports sending trace and metric data to [New Relic](https://newrelic.com/)

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
* `host_override` (Optional per-data-type, see example): Overrides the host to
  which data is sent. The URL will be generated in the form:
  https://\$host/\$path. The path component **CANNOT** be overridden.

The configuration also supports configuring by data type. This is expected to be used for `host_override` cases.

Example:
```yaml
exporters:
  newrelic:
    apikey: super-secret-api-key
    timeout: 30s

    traces:
        host_override: trace-api.newrelic.com
    metrics:
        host_override: metric-api.newrelic.com
    logs:
        host_override: log-api.newrelic.com
```

## Find and use your data

Once the exporter is sending data you can start to explore your data in New Relic:

- Metric data: see [Metric API docs](https://docs.newrelic.com/docs/data-ingest-apis/get-data-new-relic/metric-api/introduction-metric-api#find-data).
- Trace/span data: see [Trace API docs](https://docs.newrelic.com/docs/understand-dependencies/distributed-tracing/trace-api/introduction-trace-api#view-data).
- Log data: see [Log docs](https://docs.newrelic.com/docs/logs/log-management/ui-data/explore-your-data-log-analytics)

For general querying information, see:

- [Query New Relic data](https://docs.newrelic.com/docs/using-new-relic/data/understand-data/query-new-relic-data)
- [Intro to NRQL](https://docs.newrelic.com/docs/query-data/nrql-new-relic-query-language/getting-started/nrql-syntax-clauses-functions)
