# Azure Event Hub Receiver

| Status                   |           |
| ------------------------ |-----------|
| Stability                | [alpha]   |
| Supported pipeline types | logs      |
| Distributions            | [contrib] |

## Overview
Azure resources and services can be
[configured](https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/diagnostic-settings)
to send their logs to an Azure Event Hub. The Azure Event Hub receiver pulls logs from an Azure
Event Hub, transforms them, and pushes them through the collector pipeline.

## Configuration

### connection (Required)
A string describing the connection to an Azure event hub.

### partition (Optional)
The partition to watch. If empty, it will watch explicitly all partitions.

Default: ""

### offset (Optional)
The offset at which to start watching the event hub. If empty, it starts with the latest offset.

Default: ""

### encoding (Optional)
Determines how to encode the Event Hub messages into OpenTelemetry logs. See the "Encoding"
section below for details.

Default: "raw"

Example:

```yaml
receivers:
  azureeventhub:
    connection: Endpoint=sb://namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=superSecret1234=;EntityPath=hubName
    partition: foo
    offset: "1234-5566"
```

This component can persist its state using the [storage extension].

## Encoding

### raw

The "raw" encoding maps the AMQP properties and data into the
attributes and body of an OpenTelemetry LogRecord, respectively.
The body is represented as a raw byte array.

### data

The "data" encoding extracts the Azure log records from the AMQP
message data, parses them, and maps the fields to OpenTelemetry
attributes. The table below summarizes the mapping between the 
[Azure common log format](https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/resource-logs-schema)
and the OpenTelemetry attributes.


| Azure                            | OpenTelemetry                          | 
|----------------------------------|----------------------------------------|
| time (required)                  | time_unix_nano (field)                 | 
| resourceId (required)            | azure.resource.id (attribute)          | 
| tenantId (required, tenant logs) | azure.tenant.id (attribute)            | 
| operationName (required)         | azure.operation.name (attribute)       |
| operationVersion (optional)      | azure.operation.version (attribute)    | 
| category (optional)              | azure.category (attribute)             | 
| resultType (optional)            | azure.result.type (attribute)          | 
| resultSignature (optional)       | azure.result.signature (attribute)     | 
| resultDescription (optional)     | azure.result.description (attribute)   | 
| durationMs (optional)            | azure.duration (attribute)             | 
| callerIpAddress (optional)       | net.sock.peer.addr (attribute)         | 
| correlationId (optional)         | trace_id (field)                       | 
| identity (optional)              | azure.identity (attribute, nested)     |
| Level (optional)                 | severity_number, severity_text (field) | 
| location (optional)              | cloud.region (attribute)               | 
| —                                | cloud.provider (attribute)             | 
| properties (optional)            | azure.properties (attribute, nested)   | 


[alpha]: https://github.com/open-telemetry/opentelemetry-collector#alpha
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
[storage extension]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/storage
