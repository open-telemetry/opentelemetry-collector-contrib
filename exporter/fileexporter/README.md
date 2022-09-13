# File Exporter

| Status                   |                       |
| ------------------------ | --------------------- |
| Stability                | [alpha]               |
| Supported pipeline types | traces, metrics, logs |
| Distributions            | [core], [contrib]     |

Exporter supports the following features：

+ Support for writing pipeline data to a JSON file.
+ Support for self-rotate log files.

The data is written in.[Protobuf JSON encoding](https://developers.google.com/protocol-buffers/docs/proto3#json). using [OpenTelemetry protocol](https://github.com/open-telemetry/opentelemetry-proto).



Please note that there is no guarantee that exact field names will remain stable.
This intended for primarily for debugging Collector without setting up backends.

## Getting Started

The following settings are required:

- `path` (no default): where to write information.
- `rolling_logger_options` (default): an option to self-rotate log files.
    - max_size:   the maximum size in megabytes of the log file before it gets rotated. It defaults to 100 megabytes.
    - max_age:   the maximum number of days to retain old log files based on the  timestamp encoded in their filename.The default is not to remove old log files based on age.
    - max_backups:  the maximum number of old log files to retain.  The default is to retain all old log files
    - localtime : determines if the time used for formatting the timestamps in backup files is the computer's local time.  The default is to use UTC time.


Example:

```yaml
exporters:
  file:
    path: ./file.json
  file/2:
    path: ./filename.json
    rolling_logger_options:
      max_size: 10
      max_age: 3
      max_backups: 3
      localtime: true
```


[alpha]:https://github.com/open-telemetry/opentelemetry-collector#alpha
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
[core]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol