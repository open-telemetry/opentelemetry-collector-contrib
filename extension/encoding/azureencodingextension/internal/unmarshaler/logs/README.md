# Azure Resource Logs transformation rules

## General transformation rules

Transformation of Azure Resource Log records happened based on Category defined in incoming log record (`category` or `type` field) using mappings described in this document. Mapping are defined to be OpenTelemetry SemConv compatible as much as possible.

### Unknown/Unsupported Azure Resource Log record Category

For logs Category that conform [common Azure Resource Logs schema](https://learn.microsoft.com/en-us/azure/azure-monitor/platform/resource-logs-schema),
but doesn't have mapping for specific Category in this extension following rules will be applied:

* Common known fields are parsed according to [common map below](#common-fields-available-in-all-categories)
* If `properties` field is parsable JSON - all parsed attributes are put as is into Log Attributes (except for `message` - goes to Body, `correlationId` and `duration` - goes to Log Attributes according to map below)
* If `properties` field is primitive value (string, number, bool, etc.) - it will be stored into `azure.properties` Log Attribute
* If non of above is possible - `properties` will be stored as-is to Log Body

### Unparsable Azure Resource Log record

In case of parsing or transformation failure - original Azure Resource Log record
will be saved as-is (original JSON string representation) into OpenTelemetry log.Body and error will be logged.

This approach allows you to try to parse or transform Azure Resource Log record later
in OpenTelemetry Collector pipeline (for example, using `transformprocessor`) or in log Storage if applicable.

## Common fields, available in all Categories

| Azure                 | Open Telemetry |
|-----------------------|----------------|
| `time`, `timestamp`   | `log.timestamp` |
| `resourceId`          | `cloud.resource_id` (resource attribute) |
| `tenantId`            | `azure.tenant.id` (resource attribute) |
| `location`            | `cloud.region` (resource attribute) |
| `operationName`       | `azure.operation.name` (log attribute) |
| `operationVersion`    | `azure.operation.version` (log attribute) |
| `category`, `type`    | `azure.category` (log attribute) |
| `resultType`          | `azure.result.type` (log attribute) |
| `resultSignature`     | `azure.result.signature` (log attribute) |
| `resultDescription`   | `azure.result.description` (log attribute) |
| `durationMs`          | `azure.duration` (log attribute) |
| `callerIpAddress`     | `network.peer.address` (log attribute) |
| `correlationId`       | `azure.correlation_id` (log attribute) |
| `identity`            | `azure.identity` (log attribute) |
| `Level`               | `log.SeverityNumber` |
| `properties`          | see mapping for each Category below |
