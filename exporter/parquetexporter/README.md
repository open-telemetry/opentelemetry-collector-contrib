# Parquet File Exporter

| Status                   |                      |
| ------------------------ |----------------------|
| Stability                | [beta]               |
| Supported pipeline types | trace, logs, metrics |
| Distributions            | [contrib]            |

Sends pipeline data to Parquet files.

> :construction: This receiver is in beta and configuration fields are subject to change.

## Configuration

The following configuration options are required:

- `path` (no default): Export Parquet file path.

The following configuration options can also be configured:

TODO

Example:

```yaml
exporters:
  parquet:
    path: /var/output/log.parquet
```

The full list of settings exposed for this exporter are documented [here](config.go)
with detailed sample configurations [here](testdata/config.yaml).

[beta]:https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib