# LogicMonitor Exporter
This exporter supports sending logs and traces data to [Logicmonitor](https://www.logicmonitor.com/).

## Configuration Options
The following configuration options are supported:

`url (required)`: Logicmonitor's endpoint to send traces and logs.
`apitoken` : ApiToken of Logicmonitor
`headers`: Headers of POST requests

## Example
```yaml
  exporters:
    lmexporter:
      url: "https://example.logicmonitor.com/rest"
      apitoken:
        access_id: "12345"
        access_key: "abc$45fg"
```
Bearer token can alternatively be passed for authorizing ingestion to LM platform.

```yaml
  exporters:
    lmexporter:
      url: "https://example.logicmonitor.com/rest"
      headers:
        Authorization: Bearer <bearer token>
```
