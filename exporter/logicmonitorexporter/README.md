# LogicMonitor Exporter

| Status                   |                         |
| ------                   | ------                  |
| Stability                | traces [alpha](https://github.com/open-telemetry/opentelemetry-collector#alpha)              |
|                          | logs [alpha](https://github.com/open-telemetry/opentelemetry-collector#alpha)                |
| Supported pipeline types | traces, logs             |
| Distributions            | [contrib](https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib)                                     |


This exporter supports sending traces & logs to [Logicmonitor](https://www.logicmonitor.com/).

## Configuration Options
The following configuration options are supported:

`endpoint (required)`: The target base URL to send data to (e.g.: https://<company_name>.logicmonitor.com/rest). For logs, "/log/ingest" path will be appended by default.\
`api_token` : API Token of Logicmonitor

## Prerequisite
Below environment variable must be provided

| Key                  | Value        |
| ------               | ------       |
| LOGICMONITOR_ACCOUNT | Company name |

**NOTE**: For ingesting data into the Logicmonitor, either its API Token or Bearer Token is required.

## Example
##### Ingestion through API Token
Pass `access_id` and `access_key` through config.yaml as shown in example    ***OR***  
Set the environment variables `LOGICMONITOR_ACCESS_ID` and `LOGICMONITOR_ACCESS_KEY`
```yaml
  exporters:
    logicmonitor:
      endpoint: "https://<company_name>.logicmonitor.com/rest"
      api_token:
        access_id: "<access_id of logicmonitor>"
        access_key: "<access_key of logicmonitor>"
```
##### Ingestion through Bearer token

Pass bearer token as Authorization headers through config.yaml as shown in example ***OR***
Set the environment variable `LOGICMONITOR_BEARER_TOKEN`
```yaml
  exporters:
    logicmonitor:
      endpoint: "https://<company_name>.logicmonitor.com/rest"
      headers:
        Authorization: Bearer <bearer token of logicmonitor>
```
