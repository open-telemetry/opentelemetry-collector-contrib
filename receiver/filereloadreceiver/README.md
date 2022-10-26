# File Reload Receiver

| Status                   |               |
|--------------------------|---------------|
| Stability                | [alpha]       |
| Supported pipeline types | metrics, logs |
| Distributions            | [contrib]     |

## Description

This receiver periodically reload configs on the disk, supported receivers and processors can be defined in a partial
pipeline.

## Configuration

The following settings are required:

- `path` (default = `reloader.d/*yml`): glob pattern of configs to be reloaded.

The `reload` section is optional:

- `period` (default = `1m`): reload period

## Example

The OTEL collector config

```yaml
receivers:
  file_reloader/metrics:
    path: /etc/otel/*-metrics.yml
  file_reloader/logs:
    path: /etc/otel/*-logs.yml
    reload:
      period: 10s

exporters:
  otlp:

service:
  pipelines:
    metrics:
      receivers: [ file_reloader/metrics ]
      exporters: [ otlp ]
    logs:
      receivers: [ file_reloader/logs ]
      exporters: [ otlp ]
```

Sample metrics config

```yaml
receivers:
  prometheus:
    config:
      scrape_configs:
        - job_name: 'demo'
          scrape_interval: 5s

partial_pipelines:
  logs:
    receivers:
      - prometheus
```

Sample log config

```yaml
receivers:
  filelog:
    include:
      - /var/log/message

partial_pipelines:
  logs:
    receivers:
      - filelog
```

## Internal Metrics

The following internal metrics are recorded by this receiver:

| Metric                         | Description                  |
|--------------------------------|------------------------------|
| `reloads_total`                | total reloads                |
| `file_changes_total`           | total file changes detected  |
| `create_pipeline_errors_total` | total create pipeline errors |
| `started_pipelines_total`      | total started pipelines      |
| `stopped_pipelines_total`      | total stopped pipelines      |
| `active_pipelines`             | current active pipelines     |

[alpha]: https://github.com/open-telemetry/opentelemetry-collector#alpha

[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
