# Active Directory Domain Services Receiver

The `active_directory_ds` receiver scrapes metric relating to an Active Directory domain controller using the Windows Performance Counters.

Supported pipeline types: `metrics`

## Configuration
The following settings are optional:
- `metrics` (default: see `DefaultMetricsSettings` [here](./internal/metadata/generated_metrics_v2.go)): Allows enabling and disabling specific metrics from being collected in this receiver.
- `collection_interval` (default = `10s`): The interval at which metrics are emitted by this receiver.

Example:
```yaml
receivers:
  active_directory_ds:
    collection_interval: 10s
    metrics:
      # Disable the active_directory.ds.replication.network.io metric from being emitted 
      active_directory.ds.replication.network.io: false
```

The full list of settings exposed for this receiver are documented [here](./config.go) with detailed sample configurations [here](./testdata/config.yaml).

## Metrics

Details about the metrics produced by this receiver can be found in [metadata.yaml](./metadata.yaml)
