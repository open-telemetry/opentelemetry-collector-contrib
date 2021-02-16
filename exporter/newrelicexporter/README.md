# New Relic Exporter

This exporter supports sending trace and metric data to [New Relic](https://newrelic.com/)

> Please review the Collector's [security
> documentation](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/security.md),
> which contains recommendations on securing sensitive information such as the
> API key required by this exporter.

## Configuration

The following configuration options are supported:

* `apikey` (Required): Your New Relic [Insights Insert API Key](https://docs.newrelic.com/docs/insights/insights-data-sources/custom-data/send-custom-events-event-api#register).
* `timeout` (Optional): Amount of time spent attempting a request before abandoning and dropping data. Default is 15 seconds.
* `common_attributes` (Optional): Attributes to apply to all metrics sent.
* `metrics_url_override` (Optional): Overrides the endpoint to send metrics.
* `spans_url_override` (Optional): Overrides the endpoint to send spans.

Example:

```yaml
exporters:
    newrelic:
        apikey: super-secret-api-key
        timeout: 30s
        common_attributes:
          server: prod-server-01
          ready_to_rock: true
          volume: 11
```


## Find and use your data

Once the exporter is sending data you can start to explore your data in New Relic:

- Metric data: see [Metric API docs](https://docs.newrelic.com/docs/data-ingest-apis/get-data-new-relic/metric-api/introduction-metric-api#find-data).
- Trace/span data: see [Trace API docs](https://docs.newrelic.com/docs/understand-dependencies/distributed-tracing/trace-api/introduction-trace-api#view-data).

For general querying information, see:

- [Query New Relic data](https://docs.newrelic.com/docs/using-new-relic/data/understand-data/query-new-relic-data)
- [Intro to NRQL](https://docs.newrelic.com/docs/query-data/nrql-new-relic-query-language/getting-started/nrql-syntax-clauses-functions)
