# Azure Monitor Exporter

This exporter sends trace data to [Azure Monitor](https://docs.microsoft.com/en-us/azure/azure-monitor/).

## Configuration

The following settings are required:

- `instrumentation_key` (no default): Application Insights instrumentation key, which can be found in the Application Insights resource in the Azure Portal.

The following settings can be optionally configured:

- `endpoint` (default = "https://dc.services.visualstudio.com/v2/track"): The endpoint URL where data will be submitted.
- `maxbatchsize` (default = 1024): The maximum number of telemetry items that can be submitted in each request. If this many items are buffered, the buffer will be flushed before `maxbatchinterval` expires.
- `maxbatchinterval` (default = 10s): The maximum time to wait before sending a batch of telemetry.

Example:

```yaml
exporters:
  azuremonitor:
    instrumentation_key: b1cd0778-85fc-4677-a3fa-79d3c23e0efd
```

## Attribute mapping

This exporter maps OpenTelemetry trace data to [Application Insights data model](https://docs.microsoft.com/en-us/azure/azure-monitor/app/data-model-dependency-telemetry) using the following schema.

The OpenTelemetry SpanKind determines the Application Insights telemetry type.

| OpenTelemetry SpanKind           | Application Insights telemetry type |
| -------------------------------- | ----------------------------------- |
| `CLIENT`, `PRODUCER`, `INTERNAL` | Dependency                          |
| `SERVER`, `CONSUMER`             | Request                             |

The exporter follows the semantic conventions to fill the Application Insights specific telemetry properties. The following table shows a basic mapping.

| Application Insights property | OpenTelemetry attribute                               | Default   |
| ----------------------------- | ----------------------------------------------------- | --------- |
| Request.Name                  | `http.method`, `http.route` or `rpc.system`           | span name |
| Request.Url                   | `http.scheme`, `http.host`, `http.target`             |           |
| Request.Source                | `http.client_ip` or `net.peer.name`                   |           |
| Request.ResponseCode          | `http.status_code` or `status_code`                   | `"0"`     |
| Request.Success               | `http.status_code` or `status_code`                   | `true`    |
| Dependency.Name               | `http.method`, `http.route`                           | span name |
| Dependency.Data               | `http.url` or span name or `db.statement`             |           |
| Dependency.Type               | `"HTTP"` or `rpc.system` or `db.system` or `"InProc"` |           |
| Dependency.Target             | host of `http.url` or `net.peer.name`                 |           |
| Dependency.ResultCode         | `http.status_code` or `status_code`                   | `"0"`     |
| Dependency.Success            | `http.status_code` or `status_code`                   | `true`    |

The exact mapping can be found [here](trace_to_envelope.go).

All attributes are also mapped to custom properties if they are booleans or strings and to custom measurements if they are ints or doubles.
