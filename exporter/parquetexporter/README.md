# Parquet File Exporter

Sends pipeline data to Parquet files.

Supported pipeline types: logs, metrics, traces

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