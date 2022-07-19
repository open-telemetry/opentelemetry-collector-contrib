# vCenter Receiver

| Status                   |           |
| ------------------------ |-----------|
| Stability                | [alpha]   |
| Supported pipeline types | metrics   |
| Distributions            | [contrib] |

This receiver fetches metrics from a vCenter or ESXi host running VMware vSphere APIs.

## Prerequisites

This receiver has been built to support ESXi and vCenter versions:

- 7.5
- 7.0
- 6.7

A “Read Only” user assigned to a vSphere with permissions to the vCenter server, cluster and all subsequent resources being monitored must be specified in order for the receiver to retrieve information about them.

## Configuration


| Parameter           | Default | Type             | Notes                                                                                                                                                                                                                                           |
| ------------------- | ------- | ---------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| endpoint            |         | String           | Endpoint to the vCenter Server or ESXi host that has the sdk path enabled. Required. The expected format is `<protocol>://<hostname>` <br><br> i.e: `https://vcsa.hostname.localnet`                                                            |
| username            |         | String           | Required                                                                                                                                                                                                                                        |
| password            |         | String           | Required                                                                                                                                                                                                                                        |
| tls                 |         | TLSClientSetting | Not Required. Will use defaults for [configtls.TLSClientSetting](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/configtls/README.md). By default insecure settings are rejected and certificate verification is on. |
| collection_interval | 2m      | Duration         | This receiver collects metrics on an interval. If the vCenter is fairly large, this value may need to be increased. Valid time units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`                                                              |

### Example Configuration

```yaml
receivers:
  vcenter:
    endpoint: http://localhost:15672
    username: otelu
    password: $VCENTER_PASSWORD
    collection_interval: 5m
    metrics: []
```

The full list of settings exposed for this receiver are documented [here](./config.go) with detailed sample configurations [here](./testdata/config.yaml). TLS config is documented further under the [opentelemetry collector's configtls package](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/configtls/README.md).

## Metrics

Details about the metrics produced by this receiver can be found in [metadata.yaml](./metadata.yaml) with further documentation in [documentation.md](./documentation.md)

### Feature gate configurations

#### Transition from metrics with "direction" attribute

Some host metrics reported are transitioning from being reported with a `direction` attribute to being reported with the
direction included in the metric name to adhere to the OpenTelemetry specification
(https://github.com/open-telemetry/opentelemetry-specification/pull/2617):

- `vcenter.host.disk.throughput` will become:
  - `vcenter.host.disk.throughput.read`
  - `vcenter.host.disk.throughput.write`
- `vcenter.host.disk.latency.avg` will become:
  - `vcenter.host.disk.latency.avg.read`
  - `vcenter.host.disk.latency.avg.write`
- `vcenter.host.network.throughput` will become:
  - `vcenter.host.network.throughputt.receive`
  - `vcenter.host.network.throughput.transmit`
- `vcenter.host.network.packet.errors` will become:
  - `vcenter.host.network.packet.errors.receive`
  - `vcenter.host.network.packet.errors.transmit`
- `vcenter.host.network.packet.count` will become:
  - `vcenter.host.network.packet.count.receive`
  - `vcenter.host.network.packet.count.transmit`
- `vcenter.vm.disk.latency.avg.read` will become:
  - `vcenter.vm.disk.latency.avg.read`
  - `vcenter.vm.disk.latency.avg.write`
- `vcenter.vm.network.throughput` will become:
  - `vcenter.vm.network.throughput.receive`
  - `vcenter.vm.network.throughput.transmit`
- `vcenter.vm.network.packet.count` will become:
  - `vcenter.vm.network.packet.count.receive`
  - `vcenter.vm.network.packet.count.transmit`

The following feature gates control the transition process:

- **receiver.vcenterreceiver.emitMetricsWithoutDirectionAttribute**: controls if the new metrics without
  `direction` attribute are emitted by the receiver.
- **receiver.vcenterreceiver.emitMetricsWithDirectionAttribute**: controls if the deprecated metrics with 
  `direction`
  attribute are emitted by the receiver.

##### Transition schedule:

1. v0.56.0, July 2022:

- The new metrics are available for all scrapers, but disabled by default, they can be enabled with the feature gates.
- The old metrics with `direction` attribute are deprecated with a warning.
- `receiver.vcenterreceiver.emitMetricsWithDirectionAttribute` is enabled by default.
- `receiver.vcenterreceiver.emitMetricsWithoutDirectionAttribute` is disabled by default.

2. v0.58.0, August 2022:

- The new metrics are enabled by default, deprecated metrics disabled, they can be enabled with the feature gates.
- `receiver.vcenterreceiver.emitMetricsWithDirectionAttribute` is disabled by default.
- `receiver.vcenterreceiver.emitMetricsWithoutDirectionAttribute` is enabled by default.

3. v0.60.0, September 2022:

- The feature gates are removed.
- The new metrics without `direction` attribute are always emitted.
- The deprecated metrics with `direction` attribute are no longer available.

[alpha]: https://github.com/open-telemetry/opentelemetry-collector#alpha
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
