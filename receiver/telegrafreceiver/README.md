# Telegraf Receiver

Telegraf receiver for ingesting metrics from various [input plugins][input_plugins]
into otc pipeline.

Supported pipeline types: metrics

Use case: user configures telegraf input plugins in config for ingestion and otc
processors and exporters for data processing and export.

> :construction: This receiver is currently in **BETA** and is considered **unstable**.

[input_plugins]: https://github.com/influxdata/telegraf/tree/master/plugins/inputs

## Configuration

The following settings are required:

- `agent_config`: Telegraf config. For now it allows to provide agent and input
  plugins configuration. One can refer to
  [telegraf configuration docs][telegraf_config_docs] for full list of
  configuration options.

The Following settings are optional:

- `separate_field` (default value is `false`): Specify whether metric field
  should be added separately as data point label.

Example:

```yaml
receivers:
  telegraf:
    separate_field: false
    agent_config: |
      [agent]
        interval = "2s"
        flush_interval = "3s"
      [[inputs.mem]]
```

The full list of settings exposed for this receiver are documented in
[config.go](./config.go).

[telegraf_config_docs]: https://github.com/influxdata/telegraf/blob/master/docs/CONFIGURATION.md

## Limitations

With its current implementation Telegraf receiver has the following limitations:

- only input plugins can be configured in telegraf agent confugration section
  (apart from agent's configuration itself). That means that metrics go straight
  from input plugin to the receiver for translation (into otc data model) without
  any processing
- ony `telegraf.Gauge` metric data is supported, which translated (loosly) into
  `pdata.MetricDataTypeDoubleGauge` and `pdata.MetricDataTypeIntGauge` depending
  on the underlying data type
