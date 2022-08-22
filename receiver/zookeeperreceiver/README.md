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

There is a proposal to change some zookeeper metrics from being reported with a `direction` attribute to being 
reported with the direction included in the metric name.

- `zookeeper.packet.count` will become:
  - `zookeeper.packet.received.count`
  - `zookeeper.packet.sent.count`

The following feature gates control the transition process:

- **receiver.zookeeperreceiver.emitMetricsWithoutDirectionAttribute**: controls if the new metrics without `direction` attribute are emitted by the receiver.
- **receiver.zookeeperreceiver.emitMetricsWithDirectionAttribute**: controls if the deprecated metrics with `direction` attribute are emitted by the receiver.

##### Transition schedule:

The final decision on the transition is not finalized yet. The transition is on hold until
https://github.com/open-telemetry/opentelemetry-specification/issues/2726 is resolved.

[in development]: https://github.com/open-telemetry/opentelemetry-collector#in-development
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
