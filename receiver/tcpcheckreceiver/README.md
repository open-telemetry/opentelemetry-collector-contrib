# SSH Check Receiver

| Status                   |           |
| ------------------------ |-----------|
| Stability                | [beta]    |
| Supported pipeline types | metrics   |
| Distributions            | [contrib] |

This receiver creates stats by connecting to a server.

## Prerequisites


## Configuration

The following settings are required:
- `endpoint`
- `transport`

The following settings are optional:

- `collection_interval` (default = `60s`): This receiver collects metrics on an interval. Valid time units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`.

### Example Configuration

```yaml
receivers:
  tcpcheck:
    endpoint: localhost:2222
    transport: tcp
```

The full list of settings exposed for this receiver are documented [here](config.go) with detailed sample configurations [here](testdata/config.yaml). 

## Metrics

Details about the metrics produced by this receiver can be found in [metadata.yaml](metadata.yaml)

[beta]: https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
