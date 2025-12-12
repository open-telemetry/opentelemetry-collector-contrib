# Azure Resource Logs transformation rules

## General transformation rules

Transformation of Azure Resource Log records happened based on Category defined in incoming log record (`category` or `type` field) using mappings described in this document.
Mapping are defined to be OpenTelemetry SemConv compatible as much as possible.

If any of the expected field is not present in incoming JSON record or has an empty string value (i.e. "") - it will be ignored.

### Unknown/Unsupported Azure Resource Log record Category

For logs Category that conform [common Azure Resource Logs schema](https://learn.microsoft.com/en-us/azure/azure-monitor/platform/resource-logs-schema),
but doesn't have mapping for specific Category in this extension following rules will be applied:

* Common known fields are parsed according to [common map below](#common-fields-available-in-all-categories)
* If `properties` field is parsable JSON - all parsed attributes are put as is into Log Attributes (except for `message` - goes to Body, `correlationId` and `duration` - goes to Log Attributes according to map below)
* If `properties` field couldn't be parsed as JSON - it will be stored into `azure.properties` Log Attribute as string and parsing error will be logged

### Unparsable Azure Resource Log record

In case of parsing or transformation failure - original Azure Resource Log record
will be saved as-is (original JSON string representation) into OpenTelemetry log.Body and error will be logged.

This approach allows you to try to parse or transform Azure Resource Log record later
in OpenTelemetry Collector pipeline (for example, using `transformprocessor`) or in log Storage if applicable.

## Common fields, available in all Categories

| Azure                 | OpenTelemetry             | OpenTelemetry Scope |
|-----------------------|---------------------------|---------------------|
| `time`, `timestamp`   | `log.timestamp`           | Log |
| `resourceId`          | `cloud.resource_id`       | Resource Attribute |
| `tenantId`            | `azure.tenant.id`         | Resource Attribute |
| `location`            | `cloud.region`            | Resource Attribute |
| `operationName`       | `azure.operation.name`    | Log Attribute |
| `operationVersion`    | `azure.operation.version` | Log Attribute |
| `category`, `type`    | `azure.category`          | Log Attribute |
| `resultType`          | `azure.result.type`       | Log Attribute |
| `resultSignature`     | `azure.result.signature`  | Log Attribute |
| `resultDescription`   | `azure.result.description` | Log Attribute |
| `durationMs`          | `azure.duration`          | Log Attribute |
| `callerIpAddress`     | `network.peer.address`    | Log Attribute |
| `correlationId`       | `azure.correlation_id`    | Log Attribute |
| `identity`            | `azure.identity`          | Log Attribute |
| `Level`               | `log.SeverityNumber`      | Log |
| `properties`          | see mapping for each Category below | mixed |
