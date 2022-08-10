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

Some zookeeper metrics reported are transitioning from being reported with
 a `direction` attribute to being reported with the
direction included in the metric name to adhere to the OpenTelemetry specification
(https://github.com/open-telemetry/opentelemetry-specification/pull/2617):

- `zookeeper.packet.count` will become:
  - `zookeeper.packet.received.count`
  - `zookeeper.packet.sent.count`

The following feature gates control the transition process:

- **receiver.zookeeperreceiver.emitMetricsWithoutDirectionAttribute**: controls if the new metrics without `direction` attribute are emitted by the receiver.
- **receiver.zookeeperreceiver.emitMetricsWithDirectionAttribute**: controls if the deprecated metrics with `direction` attribute are emitted by the receiver.

##### Transition schedule:

1. v0.57.0, July 2022:

- The new metrics are available for all scrapers, but disabled by default, they can be enabled with the feature gates.
- The old metrics with `direction` attribute are deprecated with a warning.
- `receiver.zookeeperreceiver.emitMetricsWithDirectionAttribute` is enabled by default.
- `receiver.zookeeperreceiver.emitMetricsWithoutDirectionAttribute` is disabled by default.

2. v0.58.0, August 2022:

- The new metrics are enabled by default, deprecated metrics disabled, they can be enabled with the feature gates.
- `receiver.zookeeperreceiver.emitMetricsWithDirectionAttribute` is disabled by default.
- `receiver.zookeeperreceiver.emitMetricsWithoutDirectionAttribute` is enabled by default.

3. v0.60.0, September 2022:

- The feature gates are removed.
- The new metrics without `direction` attribute are always emitted.
- The deprecated metrics with `direction` attribute are no longer available.

[in development]: https://github.com/open-telemetry/opentelemetry-collector#in-development
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
