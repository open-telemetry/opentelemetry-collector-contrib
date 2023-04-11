# File Exporter

| Status                   |                       |
| ------------------------ | --------------------- |
| Stability                | [alpha]               |
| Supported pipeline types | traces, metrics, logs |
| Distributions            | [core], [contrib]     |

Exporter supports the following features：

+ Support for writing pipeline data to a file.

+ Support for rotation of telemetry files.

+ Support for compressing the telemetry data before exporting.


Please note that there is no guarantee that exact field names will remain stable.
This intended for primarily for debugging Collector without setting up backends.

The official [opentelemetry-collector-contrib container](https://hub.docker.com/r/otel/opentelemetry-collector-contrib/tags#!) does not have a writable filesystem by default since it's built using the special `from scratch` layer. As such, you will need to create a writable directory for the path, potentially by creating writable volumes or creating a custom image.

## Getting Started

The following settings are required:

- `path` [no default]: where to write information.

The following settings are optional:

- `rotation` settings to rotate telemetry files.

  - max_megabytes:  [default: 100]: the maximum size in megabytes of the telemetry file before it is rotated.
  - max_days: [no default (unlimited)]: the maximum number of days to retain telemetry files based on the timestamp encoded in their filename.
  - max_backups: [default: 100]: the maximum number of old telemetry files to retain.
  - localtime : [default: false (use UTC)] whether or not the timestamps in backup files is formatted according to the host's local time.

- `format`[default: json]: define the data format of encoded telemetry data. The setting can be overridden with `proto`.
- `compression`[no default]: the compression algorithm used when exporting telemetry data to file. Supported compression algorithms:`zstd`
- `flush_interval`[default: 1s]: `time.Duration` interval between flushes. See [time.ParseDuration](https://pkg.go.dev/time#ParseDuration) for valid formats. NOTE: a value without unit is in nanoseconds.

## File Rotation
Telemetry data is exported to a single file by default.
`fileexporter` only enables file rotation when the user specifies `rotation:` in the config. However, if specified, related default settings would apply.

Telemetry is first written to a file that exactly matches the `path` setting. 
When the file size exceeds `max_megabytes` or age exceeds `max_days`, the file will be rotated.

When a file is rotated, **it is renamed by putting the current time in a timestamp**
in the name immediately before the file's extension (or the end of the filename if there's no extension).
**A new telemetry file will be created at the original `path`.**

For example, if your `path` is `data.json` and rotation is triggered, this file will be renamed to `data-2022-09-14T05-02-14.173.json`, and a new telemetry file created with `data.json`

## File Compression
Telemetry data is compressed according to the `compression` setting.
`fileexporter` does not compress data by default. 

Currently, `fileexporter` support the `zstd` compression algorithm, and we will support more compression algorithms in the future.

##  File Format 

Telemetry data is encoded according to the `format` setting and then written to the file.

When `format` is json and `compression` is none , telemetry data is written to file in JSON format. Each line in the file is a JSON object.

Otherwise, when using `proto` format or any kind of encoding, each encoded object is preceded by 4 bytes (an unsigned 32 bit integer) which represent the number of bytes contained in the encoded object.When we need read the messages back in, we read the size, then read the bytes into a separate buffer, then parse from that buffer.


## Example:

```yaml
exporters:
  file/no_rotation:
    path: ./foo

  file/rotation_with_default_settings:
    path: ./foo
    rotation:

  file/rotation_with_custom_settings:
    path: ./foo
    rotation:
      max_megabytes: 10
      max_days: 3
      max_backups: 3
      localtime: true
    format: proto
    compression: zstd

  file/flush_every_5_seconds:
    path: ./foo
    flush_interval: 5
```


[alpha]:https://github.com/open-telemetry/opentelemetry-collector#alpha
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
[core]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol
