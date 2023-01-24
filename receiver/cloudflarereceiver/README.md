# Cloudflare Receiver

| Status                   |           |
|--------------------------|-----------|
| Stability                | [alpha]   |
| Supported pipeline types | logs      |
| Distributions            | [contrib] |


This Cloudflare receiver uses Cloudflare's [LogPull API](https://developers.cloudflare.com/logs/logpull/) to consume request logs over HTTP from the Cloudflare logs aggregation system. This receiver uses [Cloudflare-go](https://github.com/cloudflare/cloudflare-go/) to create an authenticated client connection to Cloudflare.
## Getting Started

Cloudflare uses [LogPull API](https://developers.cloudflare.com/logs/logpull/) which is their enterprise solution. To reach Cloudflare's HTTP endpoint, an authentication header is required that is either in the form of a X-Auth-Email and X-Auth-Key combination or an API Token.

## Configuration

### Top Level Parameters

| Parameter       | Notes        | type     | Description                                                                                                                         |
|-----------------|--------------|----------|-------------------------------------------------------------------------------------------------------------------------------------|
| `auth`          | *required*   | `Auth`   | The authentication headers required to accesses Cloudflare endpoint.                                                                |
| `poll_interval` | `default=1m` | duration | The duration waiting in between requests.                                                                                           |
| `zone`          | *required*   | string   | The Cloudflare zone ID.                                                                                                             |
| `logs`          | *optional*   | `Logs`   | Configuration for Logs ingestion of this receiver                                                                                   |
| `storage`       | *optional*   | string   | The component ID of a storage extension. If specified, the extension will ensure logs are not duplicated after a collector restart. |

### Auth Parameters

| Parameter | Notes       | type   | Description                                                      |
|-----------|-------------|--------|------------------------------------------------------------------|
| `email`   | *see below* | string | The Cloudflare account email address associated with the domain. |
| `key`     | *see below* | string | The Cloudflare API key.                                          |
| `token`   | *see below* | string | The API Token with logs edit permission used for authentication. |

**Note**  Either an email(X-Auth-Email) and key(X-Auth-Key) or a token(API Token) are required to collect Cloudflare logs.

### Logs Parameters

| Parameter | Notes                                                                                                                          | type                  | Description                                                             |
|-----------|--------------------------------------------------------------------------------------------------------------------------------|-----------------------|-------------------------------------------------------------------------|
| `count`   | `default=0`                                                                                                                    | int                   | The amount of logs that may be returned. When 0, all logs are returned. |
| `sample`  | `default=1.0`                                                                                                                  | int                   | The sample rate of logs from 0.001 to 1.0, where 1.0 is 100%.           |
| `fields`  | default=[default fields](https://developers.cloudflare.com/logs/logpull/understanding-the-basics/#format-of-the-data-returned) | `list of Field names` | Fields are a list of log fields that are returned.                      |

The fields parameter defaults to collecting the default fields, which include ClientIP, ClientRequestHost, ClientRequestMethod, ClientRequestURI, EdgeEndTimestamp, EdgeResponseBytes, EdgeResponseStatus, EdgeStartTimestamp, RayID. A list of all fields can be found [here](https://developers.cloudflare.com/logs/reference/log-fields/zone/http_requests).

*note* The field values `EdgeEndTimestamp` and `EdgeResponseStatus` will always be used to indicate the log timestamp and severity number/text.

#### Sample Configuration

With email and key:
```yaml
cloudflare:
  auth:
    email: otel@email.com
    key: abc123
  zone: 023e105f4ecef8ad9ca31a8372d0c353
```

With Token and log fields:
```yaml
cloudflare:
  auth:
    email: otel@email.com
    key: abc123
  zone: 023e105f4ecef8ad9ca31a8372d0c353
  log:
    count: 10
    sample: .9
```

[alpha]:https://github.com/open-telemetry/opentelemetry-collector#alpha
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
