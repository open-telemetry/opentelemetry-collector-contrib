# LogicMonitor Exporter

| Status                   |                         |
| ------                   | ------                  |
| Stablilty                | traces [development](https://github.com/open-telemetry/opentelemetry-collector#development)  |
|                          | logs [development](https://github.com/open-telemetry/opentelemetry-collector#development)                 |
| Supported pipeline types | traces,logs             |
| Distributions            | [contrib](https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib)                                     |


This exporter supports sending logs and traces data to [Logicmonitor](https://www.logicmonitor.com/).

## Configuration Options
The following configuration options are supported:

`endpoint (required)`: The address to send logs and traces\
`apitoken` : API Token of Logicmonitor

## Prerequisite
Below environment variable must be provided

| Key                  | Value        |
| ------               | ------       |
| LOGICMONITOR_ACCOUNT | Company name |

## Example
Ingestion through API Token

```yaml
  exporters:
    logicmonitor:
      endpoint: "https://<company_name>.logicmonitor.com/rest"
      apitoken:
        access_id: "<access_id of logicmonitor>"
        access_key: "<access_key of logicmonitor>"
```
OR 

Ingestion through Bearer token

```yaml
  exporters:
    logicmonitor:
      endpoint: "https://<company_name>.logicmonitor.com/rest"
      headers:
        Authorization: Bearer <bearer token of logicmonitor>
```