## Overview

The Datadog RUM receiver enables translation between Datadog and OpenTelemetry-compatible RUM backends.
It currently has support for Datadog's APM traces and logs.

## Configuration
Specify where the Datadog receiver should listen.

```yaml
receivers:
  datadogrum:

exporters:
    debug:
    datadog/exporter:
        api:
            site: 
            key:

connectors:
  datadog/connector:

service:
  pipelines:
    logs/rum:
      receivers: [datadogrum]
      exporters: [datadog/exporter, debug]
    traces/rum:
      receivers: [datadogrum]
      exporters: [datadog/exporter, datadog/connector]
```