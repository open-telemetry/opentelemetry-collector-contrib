# Zookeeper Receiver

| Status                   |                  |
| ------------------------ | ---------------- |
| Stability                | [in development] |
| Supported pipeline types | traces           |
| Distributions            | [contrib]        |

The Zookeeper receiver collects metrics from a Zookeeper instance, using the `mntr` command. The `mntr` 4 letter word command needs
to be enabled for the receiver to be able to collect metrics.

## Configuration

- `endpoint`: (default = `:2181`) Endpoint to connect to collect metrics. Takes the form `host:port`.
- `timeout`: (default = `10s`) Timeout within which requests should be completed.

Example configuration.

```yaml
receivers:
  zookeeper:
    endpoint: "localhost:2181"
    collection_interval: 20s
```

## Metrics

Details about the metrics produced by this receiver can be found in [metadata.yaml](./metadata.yaml) with further documentation in [documentation.md](./documentation.md)

### Feature gate configurations

#### Transition from metrics with "direction" attribute

The proposal to change metrics from being reported with a `direction` attribute has been reverted in the specification. As a result, the
following feature gates will be removed in v0.62.0:

- **receiver.zookeeperreceiver.emitMetricsWithoutDirectionAttribute**
- **receiver.zookeeperreceiver.emitMetricsWithDirectionAttribute**

For additional information, see https://github.com/open-telemetry/opentelemetry-specification/issues/2726.

[in development]: https://github.com/open-telemetry/opentelemetry-collector#in-development
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
