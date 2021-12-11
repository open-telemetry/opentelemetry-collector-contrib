This receiver will read pipeline data from JSON files. The data is written in
[Protobuf JSON
encoding](https://developers.google.com/protocol-buffers/docs/proto3#json)
using [OpenTelemetry
protocol](https://github.com/open-telemetry/opentelemetry-proto).

The receiver will watch the directory and read files. If a file is updated or added,
the receiver will read it in its entirety again.

Please note that there is no guarantee that exact field names will remain stable.
This intended for primarily for debugging Collector without setting up backends.

Supported pipeline types: traces, metrics, logs

## Getting Started

The following settings are required:

- `path` (no default): folder to watch.

Example:

```yaml
receivers:
  file:
    path: /home/ubuntu/log/
```
