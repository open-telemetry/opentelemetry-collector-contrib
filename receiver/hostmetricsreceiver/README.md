# Host Metrics Receiver

| Status                   |                   |
| ------------------------ | ----------------- |
| Stability                | [beta]            |
| Supported pipeline types | metrics           |
| Distributions            | [core], [contrib] |

The Host Metrics receiver generates metrics about the host system scraped
from various sources. This is intended to be used when the collector is
deployed as an agent.

## Getting Started

The collection interval and the categories of metrics to be scraped can be
configured:

```yaml
hostmetrics:
  collection_interval: <duration> # default = 1m
  scrapers:
    <scraper1>:
    <scraper2>:
    ...
```

The available scrapers are:

| Scraper    | Supported OSs                | Description                                            |
| ---------- | ---------------------------- | ------------------------------------------------------ |
| cpu        | All except Mac<sup>[1]</sup> | CPU utilization metrics                                |
| disk       | All except Mac<sup>[1]</sup> | Disk I/O metrics                                       |
| load       | All                          | CPU load metrics                                       |
| filesystem | All                          | File System utilization metrics                        |
| memory     | All                          | Memory utilization metrics                             |
| network    | All                          | Network interface I/O metrics & TCP connection metrics |
| paging     | All                          | Paging/Swap space utilization and I/O metrics          |
| processes  | Linux                        | Process count metrics                                  |
| process    | Linux & Windows              | Per process CPU, Memory, and Disk I/O metrics          |

### Notes

<sup>[1]</sup> Not supported on Mac when compiled without cgo which is the default.

Several scrapers support additional configuration:

### Disk

```yaml
disk:
  <include|exclude>:
    devices: [ <device name>, ... ]
    match_type: <strict|regexp>
```

### File System

```yaml
filesystem:
  <include_devices|exclude_devices>:
    devices: [ <device name>, ... ]
    match_type: <strict|regexp>
  <include_fs_types|exclude_fs_types>:
    fs_types: [ <filesystem type>, ... ]
    match_type: <strict|regexp>
  <include_mount_points|exclude_mount_points>:
    mount_points: [ <mount point>, ... ]
    match_type: <strict|regexp>
```

### Load

`cpu_average` specifies whether to divide the average load by the reported number of logical CPUs (default: `false`).

```yaml
load:
  cpu_average: <false|true>
```

### Network

```yaml
network:
  <include|exclude>:
    interfaces: [ <interface name>, ... ]
    match_type: <strict|regexp>
```

### Process

```yaml
process:
  <include|exclude>:
    names: [ <process name>, ... ]
    match_type: <strict|regexp>
  mute_process_name_error: <true|false>
```

## Advanced Configuration

### Filtering

If you are only interested in a subset of metrics from a particular source,
it is recommended you use this receiver with the
[Filter Processor](../../processor/filterprocessor).

### Different Frequencies

If you would like to scrape some metrics at a different frequency than others,
you can configure multiple `hostmetrics` receivers with different
`collection_interval` values. For example:

```yaml
receivers:
  hostmetrics:
    collection_interval: 30s
    scrapers:
      cpu:
      memory:

  hostmetrics/disk:
    collection_interval: 1m
    scrapers:
      disk:
      filesystem:

service:
  pipelines:
    metrics:
      receivers: [hostmetrics, hostmetrics/disk]
```

### Feature gate configurations

- `receiver.hostmetricsreceiver.removeDirectionAttributeNetworkMetrics`
  - Description
    - Some network metrics reported by the hostmetricsreceiver are transitioning from being reported
		with a direction attribute to being reported with the direction included in the metric name to adhere to the
		OpenTelemetry specification. Please update any monitoring this might affect.
  - Affected Metrics
    - `system.network.dropped` will become:
      - `system.network.dropped.receive`
      - `system.network.dropped.transmit`
    - `system.network.errors` will become:
      - `system.network.errors.receive`
      - `system.network.errors.transmit`
    - `system.network.io` will become:
      - `system.network.io.receive`
      - `system.network.io.transmit`
    - `system.network.packets` will become:
      - `system.network.packets.receive`
      - `system.network.packets.transmit`
  - Stages and Timeline
    - Alpha (current stage)
      - In this stage the feature gate is disabled by default and must be enabled by the user. This allows users to preemptively opt in and start using the bug fix by enabling the feature gate.
      - Collector version: v0.55.0
      - Release Date: Early July 2022
    - Beta
      - In this stage the feature gate is enabled by default and can be disabled by the user.
      - Collector version: v0.57.0
      - Release Date: Early August 2022
    - Generally Available
      - In this stage the feature gate is permanently enabled and the feature gate is no longer available for anyone.
      - Users could experience some friction in this stage, they may have to update monitoring for the affected metrics or be blocked from upgrading the collector to versions v0.59.0 and newer.
      - Collector version: v0.59.0
      - Release Date: Early September 2022
  - Usage
    - Feature gate identifiers prefixed with - will disable the gate and prefixing with + or with no prefix will enable the gate.
    - Start the otelcol with the feature gate enabled:
      - otelcol {other_arguments} --feature-gates=receiver.hostmetricsreceiver.removeDirectionAttributeNetworkMetrics
    - Start the otelcol with the feature gate disabled:
      - otelcol {other_arguments} --feature-gates=-receiver.hostmetricsreceiver.removeDirectionAttributeNetworkMetrics
  - More information:
    - https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/11815
    - https://github.com/open-telemetry/opentelemetry-specification/pull/2617
- `receiver.hostmetricsreceiver.removeDirectionAttributePagingMetrics`
  - Description
    - Some paging metrics reported by the hostmetricsreceiver are transitioning from being reported
		with a direction attribute to being reported with the direction included in the metric name to adhere to the
		OpenTelemetry specification. Please update any monitoring this might affect.
  - Affected Metrics
    - `system.paging.operations` will become:
      - `system.paging.operations.page_in`
      - `system.paging.operations.page_out`
  - Stages and Timeline
    - Alpha (current stage)
      - In this stage the feature gate is disabled by default and must be enabled by the user. This allows users to preemptively opt in and start using the bug fix by enabling the feature gate.
      - Collector version: v0.55.0
      - Release Date: Early July 2022
    - Beta
      - In this stage the feature gate is enabled by default and can be disabled by the user.
      - Collector version: v0.57.0
      - Release Date: Early August 2022
    - Generally Available
      - In this stage the feature gate is permanently enabled and the feature gate is no longer available for anyone.
      - Users could experience some friction in this stage, they may have to update monitoring for the affected metrics or be blocked from upgrading the collector to versions v0.59.0 and newer.
      - Collector version: v0.59.0
      - Release Date: Early September 2022
  - Usage
    - Feature gate identifiers prefixed with - will disable the gate and prefixing with + or with no prefix will enable the gate.
    - Start the otelcol with the feature gate enabled:
      - otelcol {other_arguments} --feature-gates=receiver.hostmetricsreceiver.removeDirectionAttributePagingMetrics
    - Start the otelcol with the feature gate disabled:
      - otelcol {other_arguments} --feature-gates=-receiver.hostmetricsreceiver.removeDirectionAttributePagingMetrics
  - More information:
    - https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/11815
    - https://github.com/open-telemetry/opentelemetry-specification/pull/2617
- `receiver.hostmetricsreceiver.removeDirectionAttributeProcessMetrics`
  - Description
    - Some process metrics reported by the hostmetricsreceiver are transitioning from being reported
		with a direction attribute to being reported with the direction included in the metric name to adhere to the
		OpenTelemetry specification. Please update any monitoring this might affect.
  - Affected Metrics
    - `process.disk.io` will become:
      - `process.disk.io.read`
      - `process.disk.io.write`
  - Stages and Timeline
    - Alpha (current stage)
      - In this stage the feature gate is disabled by default and must be enabled by the user. This allows users to preemptively opt in and start using the bug fix by enabling the feature gate.
      - Collector version: v0.55.0
      - Release Date: Early July 2022
    - Beta
      - In this stage the feature gate is enabled by default and can be disabled by the user.
      - Collector version: v0.57.0
      - Release Date: Early August 2022
    - Generally Available
      - In this stage the feature gate is permanently enabled and the feature gate is no longer available for anyone.
      - Users could experience some friction in this stage, they may have to update monitoring for the affected metrics or be blocked from upgrading the collector to versions v0.59.0 and newer.
      - Collector version: v0.59.0
      - Release Date: Early September 2022
  - Usage
    - Feature gate identifiers prefixed with - will disable the gate and prefixing with + or with no prefix will enable the gate.
    - Start the otelcol with the feature gate enabled:
      - otelcol {other_arguments} --feature-gates=receiver.hostmetricsreceiver.removeDirectionAttributeProcessMetrics
    - Start the otelcol with the feature gate disabled:
      - otelcol {other_arguments} --feature-gates=-receiver.hostmetricsreceiver.removeDirectionAttributeProcessMetrics
  - More information:
    - https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/11815
    - https://github.com/open-telemetry/opentelemetry-specification/pull/2617

[beta]: https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
[core]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol

