<h3 align="center" style="margin:0px">
 <img width="200" src="https://www.kinetica.com/wp-content/uploads/2018/08/kinetica_logo.svg" alt="Kinetica Logo"/>
</h3>
<h5 align="center" style="margin:0px">
 <a href="https://www.kinetica.com/">Website</a>
 |
 <a href="https://docs.kinetica.com/7.1/">Docs</a>
 |
 <a href="https://join.slack.com/t/kinetica-community/shared_invite/zt-1bt9x3mvr-uMKrXlSDXfy3oU~sKi84qg">Community Slack</a>
</h5>

# Kinetica OpenTelemetry Collector Exporter Plug-In

- [Overview](#overview)
- [Installation](#installation)
- [Documentation](#documentation)
- [Support](#support)
- [Contact Us](#contact-us)

## Overview

# Kinetica Opentelemetry Exporter

 This exporter could be used to as part of an `Opentelemetry` collector to persist data related to
 `logs`, `traces` and `metrics` to the `Kinetica` database.

 This component is under `alpha` status.

## Steps to build the collector binary

- Please refer to <https://github.com/open-telemetry/opentelemetry-collector/tree/main/cmd/builder> for instructions on how to get the latest binary and install locally. This link mentions - "Download the binary for your respective platform under the "Releases" page. If install an official release build, the binary is named ocb, but if you installed by using go install, it will be called builder".
- As this link describes install the Collector Builder as a package; it will be available as `ocb` after installation if installed
  from an official release.
- An example config file for building the collector binary is as follows:

```yaml
dist:
  name: otelcol-kinetica
  description: Otel collector with Kinetica exporter
  output_path: /home/kinetica/otelexporter_utils/collector-binary
  otelcol_version: 0.78.2

exporters:
  - gomod:
      github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter v0.78.0

processors:
  - gomod:
      go.opentelemetry.io/collector/processor/batchprocessor v0.78.2

receivers:
  - gomod:
      go.opentelemetry.io/collector/receiver/otlpreceiver v0.78.2
  - gomod:
      github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver v0.78.0
```

- Save the config file as e.g., `config_file.yaml`.
- Once the builder config file has been created the binary can be build using the command `ocb --config config_file.yaml`.

## Configuring the collector

The OTEL collector has a number of components at play.

1. The Receiver - this will receive the telemetry data from one of the sources which has been instrumented to produce logs, traces and metrics.
2. The Processor - the metric data received from the `Receiver`s will be passed on to this component.
3. The Exporter - this component is fed by the processor and is used to persist the metrics data to a data store (kike Kinetica) or some other component for further processing.

A sample configuration for the collector binary is as follows:

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: localhost:4317

  prometheus:
      config:
        scrape_configs:
          - job_name: 'ki_stats'
            honor_labels: true
            static_configs:
              - targets: ['172.31.32.21:9010', '172.31.32.15:9010', '172.31.32.16:9010', '172.31.32.18:9010', '172.31.33.29:9010', '172.31.32.19:9010', '172.31.32.26:9010', '172.31.32.20:9010', '172.31.32.17:9010']

processors:
  batch:

exporters:
  kinetica:
    host: http://localhost:9191/
    schema: otel
    username: admin
    password: password
    bypasssslcertcheck: true
    logconfigfile: log_config.yaml


service:
  pipelines:
    traces:
      receivers:
      - otlp
      processors:
      - batch
      exporters:
      - kinetica
    metrics:
      receivers:
      - otlp
      - prometheus
      processors:
      - batch
      exporters:
      - kinetica
    logs:
      receivers:
      - otlp
      processors:
      - batch
      exporters:
      - kinetica
```

The first section of the config file describes the components (receivers,
exporters etc.) with their configurations. The second section describes the
service details which defines the pipeline stages for logs, traces, and metrics.

Save the config file as `collector-config.yaml`.

The command to run the collector binary produced by the steps described above is:

  `collector-binary --config collector-config.yaml`

### Configuring the Kinetica Exporter

The following parameters will be necessary to define the connection for the
*Kinetica OpenTelemetry Collector Exporter*.

#### Configuration parameters for the Kinetica Exporter

| Name | Description |
|--|--|
| host | Kinetica host URL |
| schema | Kinetica OpenTelemetry schema, if omitted the default schema will be used |
| username | Kinetica User |
| password | Kinetica Password |
| bypasssslcertcheck | Whether to skip SSL certificate check or not (true or false)|
| logconfigfile | Name of the log config file |

### Configuring Logging

The logging is done using Uber zap package and `lumberjack` for rotating files based on size. Time based rotation is not supported yet. The configuration for `lumberjack` can be found here - <https://pkg.go.dev/gopkg.in/natefinch/lumberjack.v2>

#### Default Log Config file

This is included and will be used in case a user defined config file is not found. The name of the file is `config_log_zap.yaml`.

```yaml
level: 'info'
development: true
disableCaller: false
disableStacktrace: false
encoding: 'console'
encoderConfig:
  messageKey: 'msg'
  levelKey: 'level'
  timeKey: 'ts'
  nameKey: 'logger'
  callerKey: 'caller'
  functionKey: 'function'
  stacktraceKey: 'stacktrace'
  skipLineEnding: false
  lineEnding: "\n"
  levelEncoder: 'capital'
  timeEncoder: 'iso8601'
  durationEncoder: 'string'
  callerEncoder: 'full'
  nameEncoder: 'full'
  consoleSeparator: ' | '
outputPaths:
  # Implements loggin to the console
  - 'stdout'
  # Implements rolling logs using lumberjack logger; config parameters are supplied as
  # query params. Here maxSize is 10MB after which the logger rolls over; maximum
  # number of backups (maxBackups) kept is 5 and maxAge is 10 days.
  # The name of the log file in this case is "logs/kinetica-exporter.log" where the
  # "logs" directory is under the current directory on the local machine.
  - 'lumberjack://localhost/logs/kinetica-exporter.log?maxSize=10&maxBackups=5&maxAge=10'
errorOutputPaths:
  - 'stderr'
  - './logs/error_logs'
initialFields:
  app: 'kinetica-exporter'
```

## Documentation

- [Full Documentation](https://docs.kinetica.com/7.1/)
- [OpenTelemetry Docs](https://opentelemetry.io/docs/)
- [OpenTelemetry Exporter Registry](https://opentelemetry.io/ecosystem/registry/?component=exporter)

## Support

For support, you can post on
[stackoverflow](https://stackoverflow.com/questions/tagged/kinetica) under the
``kinetica`` tag or
[Slack](https://join.slack.com/t/kinetica-community/shared_invite/zt-1bt9x3mvr-uMKrXlSDXfy3oU~sKi84qg).

## Contact Us

- Ask a question on Slack:
  [Slack](https://join.slack.com/t/kinetica-community/shared_invite/zt-1bt9x3mvr-uMKrXlSDXfy3oU~sKi84qg)
- Follow on GitHub:
  [Follow @kineticadb](https://github.com/kineticadb)
- Email us:  <support@kinetica.com>
- Visit:  <https://www.kinetica.com/contact/>
