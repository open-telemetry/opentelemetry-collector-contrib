# File Receiver

| Status                   |                       |
|--------------------------|-----------------------|
| Stability                | [development]         |
| Supported pipeline types | metrics, traces, logs |
| Distributions            | [contrib]             |

The File Receiver reads the output of a
[File Exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/fileexporter),
converting that output to metrics, and sending the metrics down the pipeline.

Currently, the only file format supported is the File Exporter's JSON format. Reading compressed output, rotated files,
or telemetry other than metrics are not supported at this time.

## Getting Started

The following settings are required:

- `path` [no default]: the file in the same format as written by a File Exporter.

## Example

```yaml
receivers:
  file:
    path: ./foo
```
