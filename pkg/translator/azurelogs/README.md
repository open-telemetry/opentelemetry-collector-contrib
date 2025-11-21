# Azure Logs

The translator for Azure logs receives azure resource logs as raw data and extracts the logs in OpenTelemetry format.

Currently, it expects the azure resource logs to be coming from event hub.

### Azure CDN Access Logs

The mapping for this category is as follows:

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


### Front Door Web Application Firewall Logs

The mapping for this category is as follows:

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

### Front Door Access Logs

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

### Resource Health Logs

| Original Field (JSON)  | Log Record Attribute                           | Description                                    |
|------------------------|------------------------------------------------|------------------------------------------------|
| `title`                | `azure.resource_health.title`                  | Title of the health event.                     |
| `details`              | `azure.resource_health.details`                | Details of the health event.                   |
| `currentHealthStatus`  | `azure.resource_health.current_status`         | Current health status of the resource.         |
| `previousHealthStatus` | `azure.resource_health.previous_status`        | Previous health status of the resource.        |
| `type`                 | `azure.resource_health.type`                   | Type of the health event.                      |
| `cause`                | `azure.resource_health.cause`                  | Cause of the health event.                     |

### Service Health Logs

| Original Field (JSON)  | Log Record Attribute                           | Description                                                                           |
|------------------------|------------------------------------------------|---------------------------------------------------------------------------------------|
| `title`                | `azure.service_health.title`                   | A user-friendly, localized title for the communication.                               |
| `service`              | `azure.service_health.service`                 | The name of the service impacted by the event.                                        |
| `region`               | `azure.service_health.region`                  | The name of the region impacted by the event.                                         |
| `communication`        | `azure.service_health.communication`           | Localized details of the communication.                                               |
| `incidentType`         | `azure.service_health.incident_type`           | Identifies the category of a health event.                                            |
| `trackingId`           | `azure.service_health.tracking_id`             | The incident ID associated with this event.                                           |
| `impactStartTime`      | `azure.service_health.impact_start_time`       | The start time of the impact.                                                         |
| `impactMitigationTime` | `azure.service_health.impact_mitigation_time`  | The time when the impact was mitigated.                                               |
| `stage`                | `azure.service_health.stage`                   | Describes the current stage of the event.                                             |
| `impactedServices`     | `azure.service_health.impacted_services`       | Array of impacted services. Each service is a map containing `service_name`, `service_id`, `service_guid`, and a nested `impacted_regions` array (each with `region_name` and `region_id`). |
| `defaultLanguageTitle` | `azure.service_health.default_language_title`  | The communication title in English.                                                   |
| `defaultLanguageContent`| `azure.service_health.default_language_content`| The communication content in English.                                                 |
| `communicationId`      | `azure.service_health.communication_id`        | Unique identifier for the communication.                                              |
| `maintenanceId`        | `azure.service_health.maintenance_id`          | Unique identifier for the maintenance event.                                          |
| `maintenanceType`      | `azure.service_health.maintenance_type`        | The type of maintenance.                                                              |
| `isHIR`                | `azure.service_health.is_hir`                  | Indicates if it is a HIR event.                                                       |
| `IsSynthetic`          | `azure.service_health.is_synthetic`            | Indicates if the event is synthetic.                                                  |
| `impactType`           | `azure.service_health.impact_type`             | The type of impact.                                                                   |
| `emailTemplateId`      | `azure.service_health.email_template_id`       | ID of the email template used.                                                        |
| `emailTemplateFullVersion`| `azure.service_health.email_template_full_version`| Full version of the email template.                                                 |
| `emailTemplateLocale`  | `azure.service_health.email_template_locale`   | Locale of the email template.                                                         |
| `smsText`              | `azure.service_health.sms_text`                | Text content of the SMS notification.                                                 |
| `impactCategory`       | `azure.service_health.impact_category`         | Category of the impact.                                                               |
| `communicationRouteType`| `azure.service_health.communication_route_type`| Route type of the communication.                                                      |
| `version`              | `azure.service_health.version`                 | Version of the log schema.                                                            |
```