# Azure Event Hub Receiver

| Status                   |           |
| ------------------------ |-----------|
| Stability                | [alpha]   |
| Supported pipeline types | logs      |
| Distributions            | [contrib] |

## Overview
The Azure Event Hub receiver listens to logs emitted by Azure Event hubs.

## Configuration

### connection (Required)
A string describing the connection to an Azure event hub.

### partition (Optional)
The partition to watch. If empty, it will watch explicitly all partitions.

Default: ""

### offset (Optional)
The offset at which to start watching the event hub. If empty, it starts with the latest offset.

Default: ""

Example:

```yaml
receivers:
  azureeventhub:
    connection: Endpoint=sb://namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=superSecret1234=;EntityPath=hubName
    partition: foo
    offset: "1234-5566"
```

This component can persist its state using the [storage extension].

[alpha]: https://github.com/open-telemetry/opentelemetry-collector#alpha
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
[storage extension]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/storage
