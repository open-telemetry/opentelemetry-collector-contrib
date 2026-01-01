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

| Azure                 | OpenTelemetry                 | OpenTelemetry Scope |
|-----------------------|-------------------------------|---------------------|
| `time`, `timestamp`   | `log.timestamp`               | Log |
| `resourceId`          | `cloud.resource_id`           | Resource Attribute |
| `tenantId`            | `azure.tenant.id`             | Resource Attribute |
| `location`            | `cloud.region`                | Resource Attribute |
| `operationName`       | `azure.operation.name`        | Log Attribute |
| `operationVersion`    | `azure.operation.version`     | Log Attribute |
| `category`, `type`    | `azure.category`              | Log Attribute |
| `resultType`          | `azure.result.type`           | Log Attribute |
| `resultSignature`     | `azure.result.signature`      | Log Attribute |
| `resultDescription`   | `azure.result.description`    | Log Attribute |
| `durationMs`          | `azure.operation.duration`    | Log Attribute |
| `callerIpAddress`     | `network.peer.address`        | Log Attribute |
| `correlationId`       | `azure.correlation_id`        | Log Attribute |
| `identity`            | `azure.identity`              | Log Attribute |
| `Level`               | `log.SeverityNumber`          | Log |
| `properties`          | see mapping for each Category below | mixed |

## App Service

### App Service App Logs

| Azure "properties" Field  | OpenTelemetry                     | OpenTelemetry Scope |
|---------------------------|-----------------------------------|---------------------|
| `containerId`             | `container.id`                    | Log Attribute |
| `customLevel`             | `log.record.severity.original`    | Log Attribute |
| `exceptionClass`          | `exception.type`                  | Log Attribute |
| `host`                    | `host.name`                       | Log Attribute |
| `logger`                  | `log.record.logger`               | Log Attribute |
| `message`                 | Body                              | Log |
| `method`                  | `code.function.name`              | Log Attribute |
| `source`                  | `log.file.path`                   | Log Attribute |
| `stackTrace`              | `exception.stacktrace`            | Log Attribute |
| `webSiteInstanceId`       | `azure.app_service.instance.id`   | Log Attribute |

### App Service Audit Logs

| Azure "properties" Field  | OpenTelemetry             | OpenTelemetry Scope |
|---------------------------|---------------------------|---------------------|
| `User`                    | `user.id`                 | Log Attribute |
| `UserDisplayName`         | `user.name`               | Log Attribute |
| `UserAddress`             | `source.address`          | Log Attribute |
| `Protocol`                | `network.protocol.name`   | Log Attribute |

### App Service Authentication Logs

| Azure "properties" Field  | OpenTelemetry                         | OpenTelemetry Scope |
|---------------------------|---------------------------------------|---------------------|
| `details`                 | `azure.auth.event.details`            | Log Attribute |
| `hostName`                | `host.name`                           | Log Attribute |
| `message`                 | Body                                  | Log |
| `moduleRuntimeVersion`    | `azure.auth.module.runtime.version`   | Log Attribute |
| `siteName`                | `azure.app_service.site.name`         | Log Attribute |
| `statusCode`              | `http.response.status_code`           | Log Attribute |
| `subStatusCode`           | `azure.http.response.sub_status_code` | Log Attribute |
| `taskName`                | `azure.app_service.task.name`         | Log Attribute |

### App Service Console Logs

| Azure "properties" Field  | OpenTelemetry     | OpenTelemetry Scope |
|---------------------------|-------------------|---------------------|
| `containerId`             | `container.id`    | Log Attribute |
| `host`                    | `host.name`       | Log Attribute |

### App Service HTTP Logs

| Azure "properties" Field  | OpenTelemetry                         | OpenTelemetry Scope |
|---------------------------|---------------------------------------|---------------------|
| `CIp`                     | `client.address` + `client.port`. If unparsable -  `client.original_address` | Log Attribute |
| `ComputerName`            | `server.address`                      | Log Attribute |
| `Cookie`                  | Skipped as it may contains sensitive data, like authentication tokens | - |
| `CsBytes`                 | `http.request.size`                   | Log Attribute |
| `CsHost`                  | `http.request.header.host`            | Log Attribute |
| `CsMethod`                | `http.request.method`                 | Log Attribute |
| `CsUriQuery`              | `url.query`                           | Log Attribute |
| `CsUriStem`               | `url.path`                            | Log Attribute |
| `CsUsername`              | `user.name`                           | Log Attribute |
| `Referer`                 | `http.request.header.referer`         | Log Attribute |
| `Result`                  | Body                                  | Log |
| `ScBytes`                 | `http.response.size`                  | Log Attribute |
| `ScStatus`                | `http.response.status_code`           | Log Attribute |
| `ScSubStatus`             | `azure.http.response.sub_status_code` | Log Attribute |
| `SPort`                   | `server.port`                         | Log Attribute |
| `TimeTaken`               | `azure.request.duration`              | Log Attribute |
| `UserAgent`               | `user_agent.original`                 | Log Attribute |

### App Service IPSec Audit Logs

| Azure "properties" Field  | OpenTelemetry                             | OpenTelemetry Scope |
|---------------------------|-------------------------------------------|---------------------|
| `CIp`                     | `source.address` + `source.port`. If unparsable -  `source.original_address` | Log Attribute |
| `CsHost`                  | `http.request.header.host`                | Log Attribute |
| `details`                 | `azure.auth.event.details`                | Log Attribute |
| `Result`                  | Body                                      | Log |
| `ServiceEndpoint`         | `azure.app_service.endpoint`              | Log Attribute |
| `XAzureFDID`              | `http.request.header.x-azure-fdid`        | Log Attribute |
| `XFDHealthProbe`          | `http.request.header.x-fd-healthprobe`    | Log Attribute |
| `XForwardedFor`           | `http.request.header.x-forwarded-for`     | Log Attribute |
| `XForwardedHost`          | `http.request.header.x-forwarded-host`    | Log Attribute |

### App Service Platform Logs

| Azure "properties" Field  | OpenTelemetry             | OpenTelemetry Scope |
|---------------------------|---------------------------|---------------------|
| `containerId`             | `container.id`            | Log Attribute |
| `deploymentId`            | `azure.deployment.id`     | Log Attribute |
| `Exception`               | `exception.message`       | Log Attribute |
| `host`                    | `host.name`               | Log Attribute |
| `message`                 | Body                      | Log |
| `stackTrace`              | `exception.stacktrace`    | Log Attribute |

### App Service File Audit Logs

| Azure "properties" Field  | OpenTelemetry     | OpenTelemetry Scope |
|---------------------------|-------------------|---------------------|
| `path`                    | `file.path`       | Log Attribute |
| `process`                 | `process.title`   | Log Attribute |

## Azure CDN

### Azure CDN Access Logs

| Azure "properties" Field  | OpenTelemetry                 | OpenTelemetry Scope |
|---------------------------|-------------------------------|---------------------|
| `trackingReference`       | `azure.service.request.id`    | Log Attribute |
| `httpMethod`              | `http.request.method`         | Log Attribute |
| `httpVersion`             | `network.protocol.version`    | Log Attribute |
| `requestUri`              | `url.full` with parsed `url.scheme`, `url.domain`, `url.fragment`, `url.query`, `url.path` and `url.port`. If unparsable - only `url.original` | Log Attribute |
| `sni`                     | `tls.server.name`             | Log Attribute |
| `requestBytes`            | `http.request.size`           | Log Attribute |
| `responseBytes`           | `http.response.size`          | Log Attribute |
| `userAgent`               | `user_agent.original`         | Log Attribute |
| `clientIp`                | `client.address`              | Log Attribute |
| `clientPort`              | `client.port`                 | Log Attribute |
| `socketIp`                | `network.peer.address`        | Log Attribute |
| `timeToFirstByte`         | `azure.time_to_first_byte`    | Log Attribute |
| `timeTaken`               | `azure.request.duration`      | Log Attribute |
| `requestProtocol`         | `network.protocol.name`       | Log Attribute |
| `securityProtocol`        | `tls.protocol.name` + `tls.protocol.version`. If unparsable - `tls.protocol.original` | Log Attribute |
| `httpStatusCode`          | `http.response.status_code`   | Log Attribute |
| `pop`                     | `azure.cdn.edge.name`         | Log Attribute |
| `cacheStatus`             | `azure.cdn.cache.outcome`     | Log Attribute |
| `errorInfo`               | `exception.type`              | Log Attribute |
| `endpoint`                | `network.local.address`       | Log Attribute |
| `isReceivedFromClient`    | `network.io.direction` with value `receive` (if `true`) or `transmit` (if `false`) | Log Attribute |
| `backendHostname`         | `server.address` + `server.port`. If unparsable - `server.original_address` | Log Attribute |

## Front Door

### Front Door Web Application Firewall Logs

| Azure "properties" Field  | OpenTelemetry                         | OpenTelemetry Scope |
|---------------------------|---------------------------------------|---------------------|
| `clientIP`                | `client.address`                      | Log Attribute |
| `clientPort`              | `client.port`                         | Log Attribute |
| `socketIP`                | `network.peer.address`                | Log Attribute |
| `requestUri`              | `url.full` with parsed `url.scheme`, `url.domain`, `url.fragment`, `url.query`, `url.path` and `url.port`. If unparsable - only `url.original` | Log Attribute |
| `ruleName`                | `security_rule.name`                  | Log Attribute |
| `policy`                  | `security_rule.ruleset.name`          | Log Attribute |
| `action`                  | `security_rule.action`                | Log Attribute |
| `host`                    | `http.request.header.host`            | Log Attribute |
| `trackingReference`       | `azure.service.request.id`            | Log Attribute |
| `policyMode`              | `security_rule.ruleset.mode`          | Log Attribute |

### Front Door Health Probe Logs

| Azure "properties" Field          | OpenTelemetry                     | OpenTelemetry Scope |
|-----------------------------------|-----------------------------------|---------------------|
| `healthProbeId`                   | `azure.frontdoor.health_probe.id` | Log Attribute |
| `POP`                             | `azure.cdn.edge.name`             | Log Attribute |
| `httpVerb`                        | `http.request.method`             | Log Attribute |
| `result`                          | Body                              | Log |
| `httpStatusCode`                  | `http.response.status_code`       | Log Attribute |
| `probeURL`                        | `url.full` with parsed `url.scheme`, `url.domain`, `url.fragment`, `url.query`, `url.path` and `url.port`. If unparsable - only `url.original` | Log Attribute |
| `originName`                      | `azure.frontdoor.health_probe.origin.name` | Log Attribute |
| `originIP`                        | `server.address` + `server.port`. If unparsable - `server.original_address` | Log Attribute |
| `totalLatencyMilliseconds`        | `azure.frontdoor.health_probe.origin.latency.total` | Log Attribute |
| `connectionLatencyMilliseconds`   | `azure.frontdoor.health_probe.origin.latency.connection` | Log Attribute |
| `DNSLatencyMicroseconds`          | `azure.frontdoor.health_probe.origin.latency.dns` | Log Attribute |

### Front Door Access Logs

| Azure "properties" Field  | OpenTelemetry                 | OpenTelemetry Scope |
|---------------------------|-------------------------------|---------------------|
| `trackingReference`       | `azure.service.request.id`    | Log Attribute |
| `httpMethod`              | `http.request.method`         | Log Attribute |
| `httpVersion`             | `network.protocol.version`    | Log Attribute |
| `requestUri`              | `url.full` with parsed `url.scheme`, `url.domain`, `url.fragment`, `url.query`, `url.path` and `url.port`. If unparsable - only `url.original` | Log Attribute |
| `sni`                     | `tls.server.name`             | Log Attribute |
| `requestBytes`            | `http.request.size`           | Log Attribute |
| `responseBytes`           | `http.response.size`          | Log Attribute |
| `userAgent`               | `user_agent.original`         | Log Attribute |
| `clientIp`                | `client.address`              | Log Attribute |
| `clientPort`              | `client.port`                 | Log Attribute |
| `socketIp`                | `network.peer.address`        | Log Attribute |
| `timeToFirstByte`         | `azure.time_to_first_byte`    | Log Attribute |
| `timeTaken`               | `azure.request.duration`      | Log Attribute |
| `requestProtocol`         | `network.protocol.name`       | Log Attribute |
| `securityProtocol`        | `tls.protocol.name` + `tls.protocol.version`. If unparsable - `tls.protocol.original` | Log Attribute |
| `httpStatusCode`          | `http.response.status_code`   | Log Attribute |
| `pop`                     | `azure.cdn.edge.name`         | Log Attribute |
| `cacheStatus`             | `azure.cdn.cache.outcome`     | Log Attribute |
| `errorInfo`               | `exception.type`              | Log Attribute |
| `endpoint`                | `network.local.address`       | Log Attribute |
| `hostName`                | `http.request.header.host`    | Log Attribute |
| `securityCurves`          | `tls.curve`                   | Log Attribute |
| `securityCipher`          | `tls.cipher`                  | Log Attribute |
| `OriginIP`                | `server.address` + `server.port`. If unparsable - `server.original_address` | Log Attribute |

## Recommendation Logs

| Azure "properties" Field      | OpenTelemetry                         | OpenTelemetry Scope |
|-------------------------------|---------------------------------------|---------------------|
| `recommendationSchemaVersion` | `azure.recommendation.schema_version` | Log Attribute |
| `recommendationCategory`      | `azure.recommendation.category`       | Log Attribute |
| `recommendationImpact`        | `azure.recommendation.impact`         | Log Attribute |
| `recommendationName`          | `azure.recommendation.name`           | Log Attribute |
| `recommendationResourceLink`  | `azure.recommendation.link`           | Log Attribute |
| `recommendationType`          | `azure.recommendation.type`           | Log Attribute |
| `recommendationRisk`          | `azure.recommendation.risk`           | Log Attribute |
