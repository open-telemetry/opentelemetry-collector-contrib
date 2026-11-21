# Prometheus Normalization

[OpenTelemetry's metric semantic convention](https://github.com/open-telemetry/semantic-conventions/blob/main/docs/general/metrics.md) is not compatible with [Prometheus' own metrics naming convention](https://prometheus.io/docs/practices/naming/). This module provides centralized functions to convert OpenTelemetry metrics to Prometheus-compliant metrics. These functions are used by the following components for Prometheus:

* [prometheusreceiver](../../../receiver/prometheusreceiver/)
* [prometheusexporter](../../../exporter/prometheusexporter/)
* [prometheusremotewriteexporter](../../../exporter/prometheusremotewriteexporter/)

## Metric name

#### List of transformations to convert OpenTelemetry metrics to Prometheus metrics

| Case                                                     | Transformation                                                                                                                   | Example                                                                                   |
|----------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------|
| Unsupported characters and extraneous underscores        | Replace unsupported characters with underscores (`_`). Drop redundant, leading and trailing underscores.                         | `(lambda).function.executions(#)` → `lambda_function_executions`                          |
| Standard unit                                            | Convert the unit from [Unified Code for Units of Measure](http://ucum.org/ucum/) to Prometheus standard and append | `system.filesystem.usage` with unit `By` → `system_filesystem_usage_bytes`                |
| Non-standard unit (unit is surrounded with `{}`)         | Drop the unit                                                                                                                    | `system.network.dropped` with unit `{packets}` → `system_network_dropped`                 |
| Non-standard unit (unit is **not** surrounded with `{}`) | Append the unit, if not already present, after sanitization (all non-alphanumeric chars are dropped)                             | `system.network.dropped` with unit `packets` → `system_network_dropped_packets`           |
| Percentages (unit is `1`)                                | Append `_ratio` (for gauges only)                                                                                                | `system.memory.utilization` with unit `1` → `system_memory_utilization_ratio`             |
| Percentages (unit is `%`)                                | Replace `%` with `percent` `_percent`                                                                                            | `storage.filesystem.utilization` with unit `%` → `storage_filesystem_utilization_percent` |
| Rates (unit contains `/`)                                | Replace `/` with `per`                                                                                                           | `astro.light.speed` with unit `m/s` → `astro_light_speed_meters_per_second`               |
| Counter                                                  | Append `_total`                                                                                                                  | `system.processes.created` → `system_processes_created_total`                             |

List of standard OpenTelemetry units that will be translated to [Prometheus standard base units](https://prometheus.io/docs/practices/naming/#base-units):

| OpenTelemetry Unit | Corresponding Prometheus Unit |
| ------------------ | ----------------------------- |
| **Time**           |                               |
| `d`                | `days`                        |
| `h`                | `hours`                       |
| `min`              | `minutes`                     |
| `s`                | `seconds`                     |
| `ms`               | `milliseconds`                |
| `us`               | `microseconds`                |
| `ns`               | `nanoseconds`                 |
| **Bytes**          |                               |
| `By`               | `bytes`                       |
| `KiBy`             | `kibibytes`                   |
| `MiBy`             | `mebibytes`                   |
| `GiBy`             | `gibibytes`                   |
| `TiBy`             | `tibibytes`                   |
| `KBy`              | `kilobytes`                   |
| `MBy`              | `megabytes`                   |
| `GBy`              | `gigabytes`                   |
| `TBy`              | `terabytes`                   |
| **SI Units**       |                               |
| `m`                | `meters`                      |
| `V`                | `volts`                       |
| `A`                | `amperes`                     |
| `J`                | `joules`                      |
| `W`                | `watts`                       |
| `g`                | `grams`                       |
| **Misc.**          |                               |
| `Cel`              | `celsius`                     |
| `Hz`               | `hertz`                       |
| `%`                | `percent`                     |

> **Note**
> Prometheus also recommends using base units (no kilobytes, or milliseconds, for example) but these functions will not attempt to convert non-base units to base units.

#### List of transformations performed to convert Prometheus metrics to OpenTelemetry metrics

| Case                               | Transformation                                                         | Example                                                                         |
|------------------------------------|------------------------------------------------------------------------|---------------------------------------------------------------------------------|
| UNIT defined in OpenMetrics format | Drop the unit suffix and set it in the OpenTelemetry metric unit field | `system_network_dropped_packets` → `system_network_dropped` with `packets` unit |
| Counter                            | Drop `_total` suffix                                                   | `system_processes_created_total`→ `system_processes_created`                    |

### Simple normalization

A simple sanitization of the OpenTelemetry metric name is performed to ensure it follows Prometheus naming conventions:

* Drop unsupported characters and replace with underscores (`_`)
* Remove redundant, leading and trailing underscores
* Ensure metric name doesn't start with a digit by prefixing with an underscore

No processing of the unit is performed, and `_total` is not appended for *Counters*.

## Labels

OpenTelemetry attributes are converted to Prometheus label names according to the `translation_strategy` setting:

* `UnderscoreEscapingWithSuffixes` and `UnderscoreEscapingWithoutSuffixes` replace unsupported characters with underscores and prefix names starting with a digit with `key_`.
* `NoUTF8EscapingWithSuffixes` and `NoTranslation` preserve label names without escaping.

All four strategies preserve leading and consecutive underscores. Suffix options affect metric names, not label names.

| OpenTelemetry attribute | With underscore escaping | Without escaping |
|---|---|---|
| `name` | `name` | `name` |
| `host.name` | `host_name` | `host.name` |
| `host_name` | `host_name` | `host_name` |
| `name (of the host)` | `name__of_the_host_` | `name (of the host)` |
| `2 cents` | `key_2_cents` | `2 cents` |
| `__name` | `__name` | `__name` |
| `_name` | `_name` | `_name` |
| `a__b` | `a__b` | `a__b` |

See the [Prometheus exporter](../../../exporter/prometheusexporter/README.md) and [Prometheus remote-write exporter](../../../exporter/prometheusremotewriteexporter/README.md) documentation for configuration details.
