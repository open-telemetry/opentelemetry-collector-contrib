# Microsoft IIS Receiver

| Status                   |           |
| ------------------------ |-----------|
| Stability                | [beta]    |
| Supported pipeline types | metrics   |
| Distributions            | [contrib] |

The `iis` receiver grabs metrics about an IIS instance using the Windows Performance Counters.
Because of this, it is a Windows only receiver.

## Configuration

The following settings are optional:

- `collection_interval` (default = `10s`): The interval at which metrics should be emitted by this receiver.

Example:

```yaml
    receivers:
      iis:
        collection_interval: 10s
```

The full list of settings exposed for this receiver are documented [here](./config.go).

## Metrics

Details about the metrics produced by this receiver can be found in [documentation.md](./documentation.md)

[beta]: https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
