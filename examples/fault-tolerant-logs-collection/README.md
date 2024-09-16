## Fault tolerant log collection example

Filelog receiver's persistence can be covered by the usage of the following extensions:
- [filestorage](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/storage/filestorage) extension,
  to ensure that Collector's restarts do not affect the log collection and offset tracking.
- [exporterhelper persistent-queue](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/exporterhelper/README.md#persistent-queue),
  to ensure that Collector's restarts do not affect the delivery of the already collected logs.

A full configuration example is provided in [example config](./otel-col-config.yaml)