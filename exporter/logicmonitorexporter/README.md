# LogicMonitor Exporter
This exporter supports sending logs and traces data to [Logicmonitor](https://www.logicmonitor.com/).

## Configuration Options
The following configuration options are supported:

`url (required)`: The address to send logs and traces\
`apitoken` : API Token of Logicmonitor\
`headers`: Headers of POST requests\
`log_batching_enabled`(default = true) : The flag to enable/disable batching of logs\
`log_batching_interval`(default = 10s) : The time interval for batching of logs

## Prerequisite:
Below environment variable must be provided

| Key | Value |
| ------ | ------ |
| LOGICMONITOR_ACCOUNT | Company name |

## Example
Ingestion through API Token

```yaml
  exporters:
    logicmonitor:
      url: "https://<company_name>.logicmonitor.com/rest"
      apitoken:
        access_id: "<access_id of logicmonitor>"
        access_key: "<access_key of logicmonitor>"
```
OR 

Ingestion through Bearer token

```yaml
  exporters:
    logicmonitor:
      url: "https://<company_name>.logicmonitor.com/rest"
      headers:
        Authorization: Bearer <bearer token of logicmonitor>
```