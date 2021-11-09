# Prometheus Trace Metrics Exporter

Prometheus Trace Metrics Exporter exports Prometheus metrics on HTTP /metrics endpoint.

Example:
```yaml
exporters:
  prometheustracemetrics:
    scrape_path: "/metrics"
    scrape_listen: ":8080"
```
