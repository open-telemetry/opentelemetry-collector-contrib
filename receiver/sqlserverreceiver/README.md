# Microsoft SQL Server Receiver

The `sqlserver` receiver grabs metrics about a Microsoft SQL Server instance using the Windows Performance Counters.
Because of this, it is a Windows only receiver.

Supported pipeline types: `metrics`

## Configuration

The following settings are optional:

- `collection_interval` (default = `10s`): The internal at which metrics should be emitted by this receiver.

Example:

```yaml
    receivers:
      sqlserver:
        collection_interval: 10s
```

The full list of settings exposed for this receiver are documented [here](./config.go) with detailed sample configurations [here](./testdata/config.yaml).

## Metrics

Details about the metrics produced by this receiver can be found in [documentation.md](./documentation.md)
