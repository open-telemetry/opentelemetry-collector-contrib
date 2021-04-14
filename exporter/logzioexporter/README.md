# Logzio Exporter

This exporter supports sending trace data to [Logz.io](https://www.logz.io)

The following configuration options are supported:

* `account_token` (Required): Your logz.io account token for your tracing account.
* `metrics_token` (Optional): This is deprecated, but may be used for the OpenSearch/ElasticSearch based Metrics backend.
* `region` (Optional): Your logz.io account [region code](https://docs.logz.io/user-guide/accounts/account-region.html#available-regions). Defaults to `us`. Required only if your logz.io region is different than US.
* `custom_endpoint` (Optional): Custom endpoint, mostly used for dev or testing. This will override the region parameter.

Example:

```yaml
exporters:
  logzio:
    account_token: "LOGZIOtraceTOKEN"
    metrics_token: "LOGZIOmetricsTOKEN"
    region: "eu"
```
In order to use the Prometheus backend you must use the standard prometheusremotewrite exporter as well.

Example:

```yaml
exporters:
  prometheusremotewrite:
    endpoint: "http://myremotewriteendpoint:1234"
    headers:
      Authorization: "Bearer mybearertoken"
```

Putting these both together it would look like this in a full configuration:

```yaml
receivers:
  jaeger:
    protocols:
      thrift_http:
        endpoint: "0.0.0.0:14278"

  prometheus:
    config:
      scrape_configs:
      - job_name: 'ratelimiter'
        scrape_interval: 15s
        static_configs:
        - targets: [ "0.0.0.0:8889" ]

exporters:
  logzio:
    account_token: "LOGZIOtraceTOKEN"
    region: "us"

  prometheusremotewrite:
    endpoint: "http://myremotewriteendpoint:1234"
    headers:
      Authorization: "Bearer LOGZIOprometheusTOKEN"

service:
  pipelines:
    traces:
      receivers: [jaeger]
      exporters: [logzio]

    metrics:
      receivers: [prometheus]
      exporters: [prometheusremotewrite]
```
