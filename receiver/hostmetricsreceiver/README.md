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
  scrape_process_delay: <time>
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

#### Transition from metrics with "direction" attribute

There is a proposal to change some host metrics from being reported with a `direction` attribute to being
reported with the direction included in the metric name.

- `disk` scraper metrics:
  - `system.disk.io` will become:
    - `system.disk.io.read`
    - `system.disk.io.write`
  - `system.disk.operations` will become:
    - `system.disk.operations.read`
    - `system.disk.operations.write`
  - `system.disk.operation_time` will become:
    - `system.disk.operation_time.read`
    - `system.disk.operation_time.write`
  - `system.disk.merged` will become:
    - `system.disk.merged.read`
    - `system.disk.merged.write`
- `network` scraper metrics:
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
- `paging` scraper metrics:
  - `system.paging.operations` will become:
    - `system.paging.operations.page_in`
    - `system.paging.operations.page_out`
- `process` scraper metrics:
  - `process.disk.io` will become:
    - `process.disk.io.read`
    - `process.disk.io.write`

The following feature gates control the transition process:

- **receiver.hostmetricsreceiver.emitMetricsWithoutDirectionAttribute**: controls if the new metrics without
  `direction` attribute are emitted by the receiver.
- **receiver.hostmetricsreceiver.emitMetricsWithDirectionAttribute**: controls if the deprecated metrics with 
  `direction`
  attribute are emitted by the receiver.

##### Transition schedule:

The final decision on the transition is not finalized yet. The transition is on hold until
https://github.com/open-telemetry/opentelemetry-specification/issues/2726 is resolved.

##### Usage:

To enable the new metrics without `direction` attribute and disable the deprecated metrics, run OTel Collector with the 
following arguments:

```sh
otelcol --feature-gates=-receiver.hostmetricsreceiver.emitMetricsWithDirectionAttribute,+receiver.hostmetricsreceiver.emitMetricsWithoutDirectionAttribute
```

It's also possible to emit both the deprecated and the new metrics:

```sh
otelcol --feature-gates=+receiver.hostmetricsreceiver.emitMetricsWithDirectionAttribute,+receiver.hostmetricsreceiver.emitMetricsWithoutDirectionAttribute
```

If both feature gates are enabled, each particular metric can be disabled with the user settings, for example:

```yaml
receivers:
  hostmetrics:
    scrapers:
      paging:
        metrics:
          system.paging.operations:
            enabled: false
```

##### More information:

- https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/11815
- https://github.com/open-telemetry/opentelemetry-specification/pull/2617

[beta]: https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
[core]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol

