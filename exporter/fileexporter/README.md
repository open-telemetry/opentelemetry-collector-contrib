# File Exporter

| Status                   |                       |
| ------------------------ |-----------------------|
| Stability                | [alpha]               |
| Supported pipeline types | traces, metrics, logs |
| Distributions            | [core], [contrib]     |

Exporter supports the following features：

+ Support for writing pipeline data to a JSON file.
+ Support for encoding data using a protobuf stream in OTLP batch format described in [Add proto messages for signals data independent of OTLP protocol opentelemetry-proto#332](https://github.com/open-telemetry/opentelemetry-proto/pull/332)
+ Support for self-rotate log files.
+ Support for data compression using [Zstd](http://facebook.github.io/zstd/).


Please note that there is no guarantee that exact field names will remain stable.
This intended for primarily for debugging Collector without setting up backends.

## Getting Started

The following settings are required:

- `path` (no default): where to write information.

Example:

```yaml
exporters:
  file:
    path: ./filename.json
    proto_marshal_option: true
    zstd_option: true
    rolling_logger_options:
      max_size: 10
      max_age: 3
      max_backups: 3
      localtime: true
```


[alpha]:https://github.com/open-telemetry/opentelemetry-collector#alpha
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
[core]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol