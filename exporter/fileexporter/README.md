# File Exporter

| Status                   |                       |
| ------------------------ | --------------------- |
| Stability                | [alpha]               |
| Supported pipeline types | traces, metrics, logs |
| Distributions            | [core], [contrib]     |

Exporter supports the following features：

+ Support for writing pipeline data to a JSON file.
+ Support for rotation of telemetry files.

The data is written in.[Protobuf JSON encoding](https://developers.google.com/protocol-buffers/docs/proto3#json). using [OpenTelemetry protocol](https://github.com/open-telemetry/opentelemetry-proto).



Please note that there is no guarantee that exact field names will remain stable.
This intended for primarily for debugging Collector without setting up backends.

## Getting Started

The following settings are required:
- `path` (no default): where to write information.

The following settings are optional:
- `rotation` settings to rotate telemetry files.
    - max_megabytes:  [default: 100]: the maximum size in megabytes of the telemetry file before it is rotated.
    - max_days: [no default (unlimited)]: the maximum number of days to retain telemetry files based on the timestamp encoded in their filename.
    - max_backups: [default: 100]: the maximum number of old telemetry files to retain.
    - localtime : [default: false (use UTC)] whether or not the timestamps in backup files is formatted according to the host's local time.

## File Rotation
Telemetry is first written to a file that exactly matches the `path` setting. When the file size exceeds `max_megabytes` or age exceeds `max_days`, the file will be rotated.

When a file is rotated, **it is renamed by putting the current time in a timestamp**
in the name immediately before the file's extension (or the end of the filename if there's no extension).
**A new telemetry file will be created at the original `path`.**

For example, if your `path` is `data.json` and rotation is triggered, this file will be renamed to `data-2022-09-14T05-02-14.173.json`, and a new telemetry file created with `data.json`
## Example:

```yaml
exporters:
  file:
    path: ./file.json
  file/2:
    path: ./filename.json
    rotation:
      max_megabytes: 10
      max_days: 3
      max_backups: 3
      localtime: true
```


[alpha]:https://github.com/open-telemetry/opentelemetry-collector#alpha
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
[core]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol