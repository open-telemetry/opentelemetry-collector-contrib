# Datadog APM Receiver

| Status                   |           |
| ------------------------ | --------- |
| Stability                | [beta]   |
| Supported pipeline types | traces      |
| Distributions            | [contrib] |

## Overview
The Datadog APM Receiver accepts traces in the Datadog Trace Agent Format

### Supported Datadog APIs

- v0.3 (msgpack and json)
- v0.4 (msgpack and json)
- v0.5 (msgpack custom format)
## Configuration

Example:

```yaml
receivers:
  datadog:
    endpoint: localhost:8126
    read_timeout: 60s
```
### read_timeout (Optional)
The read timeout of the HTTP Server

Default: 60s

### HTTP Service Config

All config params here are valid as well

https://github.com/open-telemetry/opentelemetry-collector/tree/main/config/confighttp#server-configuration


[beta]:https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
