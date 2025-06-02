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