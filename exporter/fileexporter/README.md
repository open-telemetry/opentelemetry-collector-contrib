# File Exporter

| Status                   |                        |
| ------------------------ |------------------------|
| Stability                | [beta]                 |
| Supported pipeline types | traces, metrics, logs  |
| Distributions            | [contrib]              |

This exporter will write pipeline data to a JSON file. The data is written in.
[Protobuf JSON
encoding](https://developers.google.com/protocol-buffers/docs/proto3#json)
using [OpenTelemetry
protocol](https://github.com/open-telemetry/opentelemetry-proto).

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
```


[beta]:https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
