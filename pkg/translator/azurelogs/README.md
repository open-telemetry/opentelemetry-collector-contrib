# Azure Logs

The translator for Azure logs receives azure resource logs as raw data and extracts the logs in OpenTelemetry format.

Currently, it expects the azure resource logs to be coming from event hub.

## Common

### Top-level Common Schema

https://learn.microsoft.com/en-us/azure/azure-monitor/platform/resource-logs-schema#top-level-common-schema

| Original Field (JSON) | Resource Attribute  |
| --------------------- | ------------------- |
| `resourceId`          | `cloud.resource_id` |

| Original Field (JSON) | Log Record                                     |
| --------------------- | ---------------------------------------------- |
| `time`                | `timeUnixNano`                                 |
| `level`               | Decoded as `severityNumber` and `severityText` |

| Original Field (JSON) | Log Record Attribute       |
| --------------------- | -------------------------- |
| `tenantId`            | `cloud.account.id`         |
| `operationName`       | `azure.operation.name`     |
| `operationVersion`    | `azure.operation.version`  |
| `category`            | `azure.category`           |
| `resultType`          | `azure.result.type`        |
| `resultSignature`     | `azure.result.signature`   |
| `resultDescription`   | `azure.result.description` |
| `durationMs`          | `azure.duration`           |
| `callerIpAddress`     | `network.peer.address`     |
| `correlationId`       | `azure.correlation.id`     |
| `identity`            | `azure.identity`           |


### Identity

| Original Field (JSON)    | Log Record Attribute           |
| ------------------------ | ------------------------------ |
| `identity.authorization` | `azure.identity.authorization` |
| `identity.claims`        | `azure.identity.claims`        |

#### Authorization

| Original Field (JSON)                                     | Log Record Attribute                                          |
| --------------------------------------------------------- | ------------------------------------------------------------- |
| `identity.authorization.scope`                            | `azure.identity.authorization.scope`                          |
| `identity.authorization.action`                           | `azure.identity.authorization.action`                         |
| `identity.authorization.evidence.role`                    | `azure.identity.authorization.evidence.role`                  |
| `identity.authorization.evidence.roleAssignmentScope`     | `azure.identity.authorization.evidence.role.assignment.scope` |
| `identity.authorization.evidence.roleAssignmentId`        | `azure.identity.authorization.evidence.role.assignment.id`    |
| `identity.authorization.evidence.roleDefinitionId`        | `azure.identity.authorization.evidence.role.definition.id`    |
| `identity.authorization.evidence.principalId`             | `azure.identity.authorization.evidence.principal.id`          |
| `identity.authorization.evidence.principalType`           | `azure.identity.authorization.evidence.principal.type`        |

#### Claims

| Original Field (JSON)                                                | Log Record Attribute                     |
| -------------------------------------------------------------------- | ---------------------------------------- |
| `http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress` | `user.email`                             |
| `aud`                                                                | `azure.identity.audience`                |
| `iss`                                                                | `azure.identity.issuer`                  |
| `sub`                                                                | `azure.identity.subject`                 |
| `exp`                                                                | `azure.identity.not_after` (ISO 8601)    |
| `nbf`                                                                | `azure.identity.not_before` (ISO 8601)   |
| `iat`                                                                | `azure.identity.created` (ISO 8601)      |
| `http://schemas.microsoft.com/identity/claims/scope`                 | `azure.identity.scope`                   |
| `http://schemas.microsoft.com/claims/authnclassreference`            | `azure.identity.type`                    |
| `appid`                                                              | `azure.identity.application.id`          |
| `amr`                                                                | `azure.identity.auth.methods.references` |
| `http://schemas.microsoft.com/identity/claims/objectidentifier`      | `azure.identity.identifier.object`       |
| `name`                                                               | `user.name`         |
| `http://schemas.microsoft.com/identity/claims/identityprovider`      | `azure.identity.provider`                |

## Resource Logs

### Azure Content Delivery Network (CDN)

#### Azure CDN Access Logs

| Original Field (JSON)   | Log Record Attribute                                                                                                                  |
|-------------------------|---------------------------------------------------------------------------------------------------------------------------------------|
| `trackingReference`     | `azure.ref`                                                                                                                           |
| `httpMethod`            | `http.request.method`                                                                                                                 |
| `httpVersion`           | `network.protocol.version`                                                                                                            |
| `requestUri`            | `url.orginal`<br>Also parses it to get fields:<br>1.`url.scheme`<br>2.`url.fragment`<br>3.`url.query`<br>4.`url.path`<br>5.`url.port` |
| `sni`                   | `tls.server.name`                                                                                                                     |
| `requestBytes`          | `http.request.size`                                                                                                                   |
| `responseBytes`         | `http.response.size`                                                                                                                  |
| `userAgent`             | `user_agent.original`                                                                                                                 |
| `clientIp`              | `client.address`                                                                                                                      |
| `clientPort`            | `client.port`                                                                                                                         |
| `socketIp`              | `source.address`                                                                                                                      |
| `timeToFirstByte`       | `azure.time_to_first_byte`                                                                                                            |
| `timeTaken`             | `duration`                                                                                                                            |
| `requestProtocol`       | `network.protocol.name`                                                                                                               |
| `securityProtocol`      | 1. `tls.protocol.name`<br>2. `tls.protocol.version`                                                                                   |
| `httpStatusCode`        | `http.response.status_code`                                                                                                           |
| `pop`                   | `azure.pop`                                                                                                                           |
| `cacheStatus`           | `azure.cache_status`                                                                                                                  |
| `errorInfo`             | `exception.type`                                                                                                                      |
| `ErrorInfo`             | Same as `errorInfo`                                                                                                                   |
| `endpoint`              | Either:<br>1. `destination.address` if it is equal to `backendHostname`<br>2. `network.peer.address` otherwise.                       |
| `isReceivedFromClient`  | `network.io.direction`<br>- If `true`, `receive`<br>- Else, `transmit`                                                                |
| `backendHostname`       | 1. `destination.address` <br>2. `destination.port`, if any                                                                            |

### Azure Front Door

#### Front Door Web Application Firewall Logs

| Original Field (JSON) | Log Record Attribute                                                                                                                  |
|-----------------------|---------------------------------------------------------------------------------------------------------------------------------------|
| `clientIP`            | `client.address`                                                                                                                      |
| `clientPort`          | `client.port`                                                                                                                         |
| `socketIP`            | `source.address`                                                                                                                      |
| `requestUri`          | `url.orginal`<br>Also parses it to get fields:<br>1.`url.scheme`<br>2.`url.fragment`<br>3.`url.query`<br>4.`url.path`<br>5.`url.port` |
| `ruleName`            | `azure.frontdoor.waf.rule.name`                                                                                                       |
| `policy`              | `azure.frontdoor.waf.policy.name`                                                                                                     |
| `action`              | `azure.frontdoor.waf.action`                                                                                                          |
| `host`                | `http.request.header.host`                                                                                                            |
| `trackingReference`   | `azure.ref`                                                                                                                           |
| `policyMode`          | `azure.frontdoor.waf.policy.mode`                                                                                                     |

#### Front Door Access Logs

| Original Field (JSON) | Log Record Attribute                                                                                                                  |
|-----------------------|---------------------------------------------------------------------------------------------------------------------------------------|
| `trackingReference`   | `azure.ref`                                                                                                                           |
| `httpMethod`          | `http.request.method`                                                                                                                 |
| `httpVersion`         | `network.protocol.version`                                                                                                            |
| `requestUri`          | `url.orginal`<br>Also parses it to get fields:<br>1.`url.scheme`<br>2.`url.fragment`<br>3.`url.query`<br>4.`url.path`<br>5.`url.port` |
| `sni`                 | `tls.server.name`                                                                                                                     |
| `requestBytes`        | `http.request.size`                                                                                                                   |
| `responseBytes`       | `http.response.size`                                                                                                                  |
| `userAgent`           | `user_agent.original`                                                                                                                 |
| `clientIp`            | `client.address`                                                                                                                      |
| `clientPort`          | `client.port`                                                                                                                         |
| `socketIp`            | `source.address`                                                                                                                      |
| `timeToFirstByte`     | `azure.time_to_first_byte`                                                                                                            |
| `timeTaken`           | `duration`                                                                                                                            |
| `requestProtocol`     | `network.protocol.name`                                                                                                               |
| `securityProtocol`    | 1. `tls.protocol.name`<br>2. `tls.protocol.version`                                                                                   |
| `httpStatusCode`      | `http.response.status_code`                                                                                                           |
| `pop`                 | `azure.pop`                                                                                                                           |
| `cacheStatus`         | `azure.cache_status`                                                                                                                  |
| `errorInfo`           | `exception.type`                                                                                                                      |
| `ErrorInfo`           | Same as `errorInfo`                                                                                                                   |
| `endpoint`            | Either:<br>1. `destination.address` if it is equal to `hostName`<br>2. `network.peer.address` otherwise.                              |
| `hostName`            | 1. `destination.address` <br>2. `destination.port`, if any                                                                            |
| `securityCurves`      | `tls.curve`                                                                                                                           |
| `securityCipher`      | `tls.cipher`                                                                                                                          |
| `OriginIP`            | Split in:<br>1.`server.address`<br>2.`server.port`                                                                                    |

## Activity Logs

### Administrative

| Original Field (JSON)               | Log Record Attribute                  |
| ----------------------------------- | ------------------------------------- |
| `properties.entity`                 | `azure.administrative.entity`         |
| `properties.message`                | `azure.administrative.message`        |
| `properties.hierarchy`              | `azure.administrative.hierarchy`      |

### Alert

| Original Field (JSON)               | Log Record Attribute                  |
| ----------------------------------- | ------------------------------------- |
| `properties.webHookUri`             | `azure.alert.webhook.uri`             |
| `properties.ruleUri`                | `azure.alert.rule.uri`                |
| `properties.ruleName`               | `azure.alert.rule.name`               |
| `properties.description`            | `azure.alert.rule.description`        |
| `properties.threshold`              | `azure.alert.threshold`               |
| `properties.windowSize`             | `azure.alert.window_size_minutes`     |
| `properties.aggregation`            | `azure.alert.aggregation`             |
| `properties.operator`               | `azure.alert.operator`                |
| `properties.metricName`             | `azure.alert.metric.name`             |
| `properties.metricUnit`             | `azure.alert.metric.unit`             |

### Autoscale

| Original Field (JSON)               | Log Record Attribute                        |
| ----------------------------------- | ------------------------------------------- |
| `properties.description`            | `azure.autoscale.description`               |
| `properties.resourceName`           | `azure.autoscale.resource.name`             |
| `properties.oldInstancesCount`      | `azure.autoscale.instances.previous_count`  |
| `properties.newInstancesCount`      | `azure.autoscale.instances.count`           |
| `properties.lastScaleActionTime`    | `azure.autoscale.resource.last_scale`       |

### Policy

| Original Field (JSON)               | Log Record Attribute                  |
| ----------------------------------- | ------------------------------------- |
| `properties.isComplianceCheck`      | `azure.policy.compliance_check`       |
| `properties.ancestors`              | `azure.policy.ancestors`              |
| `properties.hierarchy`              | `azure.policy.hierarchy`              |

### Recommendation

| Original Field (JSON)                    | Log Record Attribute                  |
| ---------------------------------------- | ------------------------------------- |
| `properties.recommendationCategory`      | `azure.recommendation.category`       |
| `properties.recommendationImpact`        | `azure.recommendation.impact`         |
| `properties.recommendationName`          | `azure.recommendation.name`           |
| `properties.recommendationType`          | `azure.recommendation.type`           |
| `properties.recommendationSchemaVersion` | `azure.recommendation.schema_version` |
| `properties.recommendationResourceLink`  | `azure.recommendation.link`           |

### Resource Health

| Original Field (JSON)                    | Log Record Attribute                          |
| ---------------------------------------- | --------------------------------------------- |
| `properties.title`                       | `azure.resourcehealth.title`                  |
| `properties.details`                     | `azure.resourcehealth.details`                |
| `properties.currentHealthStatus`         | `azure.resourcehealth.state`                  |
| `properties.previousHealthStatus`        | `azure.resourcehealth.previous_state`         |
| `properties.type`                        | `azure.resourcehealth.type`                   |
| `properties.cause`                       | `azure.resourcehealth.cause`                  |

### Security

| Original Field (JSON)                    | Log Record Attribute                      | Notes |
| ---------------------------------------- | ----------------------------------------- | ----- |
| `properties.accountLogonId`              | `azure.security.account_logon_id`         | |
| `properties.commandLine`                 | `process.command_line`                    | |
| `properties.domainName`                  | `azure.security.domain_name`              | |
| `properties.parentProcess`               | Not mapped                                | Contains descriptive text (e.g., "unknown") rather than process name; `process.parent_pid` provides the parent process identifier |
| `properties.parentProcessId`             | `process.parent_pid`                      | |
| `properties.processId`                   | `process.pid`                             | |
| `properties.processName`                 | `process.executable.path`                 | |
| `properties.userName`                    | `process.owner`                           | |
| `properties.userSID`                     | `enduser.id`                              | |
| `properties.actionTaken`                 | `azure.security.action_taken`             | |
| `properties.severity`                    | `azure.security.severity`                 | |

### Service Health

| Original Field (JSON)                                        | Log Record Attribute                               |
| ------------------------------------------------------------ | -------------------------------------------------- |
| `properties.title`                                           | `azure.servicehealth.title`                        |
| `properties.service`                                         | `azure.servicehealth.service`                      |
| `properties.region`                                          | `azure.servicehealth.region`                       |
| `properties.communication.communicationId`                   | `azure.servicehealth.communication.id`             |
| `properties.communication.communicationBody`                 | `azure.servicehealth.communication.body`           |
| `properties.communication.routeType`                         | `azure.servicehealth.communication.route_type`     |
| `properties.incidentType`                                    | `azure.servicehealth.incident.type`                |
| `properties.trackingId`                                      | `azure.servicehealth.tracking.id`                  |
| `properties.impactStartTime`                                 | `azure.servicehealth.impact.start`                 |
| `properties.impactMitigationTime`                            | `azure.servicehealth.impact.mitigation`            |
| `properties.impactedServices`                                | `azure.servicehealth.impact.services` (array)      |
| `properties.impactedServices[].ServiceName`                  | `name` (within service object)                     |
| `properties.impactedServices[].ServiceId`                    | `id` (within service object)                       |
| `properties.impactedServices[].ServiceGuid`                  | `guid` (within service object)                     |
| `properties.impactedServices[].ImpactedRegions`              | `regions` (array within service object)            |
| `properties.impactedServices[].ImpactedRegions[].RegionName` | `name` (within region object)                      |
| `properties.impactedServices[].ImpactedRegions[].RegionId`   | `id` (within region object)                        |
| `properties.impact`                                          | `azure.servicehealth.impact.type`                  |
| `properties.category`                                        | `azure.servicehealth.impact.category`              |
| `properties.defaultLanguageTitle`                            | `azure.servicehealth.default_language.title`       |
| `properties.defaultLanguageContent`                          | `azure.servicehealth.default_language.content`     |
| `properties.stage`                                           | `azure.servicehealth.state`                        |
| `properties.maintenanceId`                                   | `azure.servicehealth.maintenance.id`               |
| `properties.maintenanceType`                                 | `azure.servicehealth.maintenance.type`             |
| `properties.isHIR`                                           | `azure.servicehealth.is_hir`                       |
| `properties.isSynthetic`                                     | `azure.servicehealth.is_synthetic`                 |
| `properties.emailTemplateId`                                 | `azure.servicehealth.email.template.id`            |
| `properties.emailTemplateFullVersion`                        | `azure.servicehealth.email.template.full_version`  |
| `properties.emailTemplateLocale`                             | `azure.servicehealth.email.template.locale`        |
| `properties.smsText`                                         | `azure.servicehealth.sms.text`                     |
| `properties.version`                                         | `azure.servicehealth.version`                      |
| `properties.argQuery`                                        | `azure.servicehealth.arg_query`                    |
| `properties.newRate`                                         | `azure.servicehealth.new_rate`                     |
| `properties.oldRate`                                         | `azure.servicehealth.old_rate`                     |
