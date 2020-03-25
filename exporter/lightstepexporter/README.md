# LightStep Exporter

This exporter supports sending trace data to [LightStep](https://www.lightstep.com)

The following configuration options are supported:

* `access_token` (Required): The access token for your LightStep project.
* `satellite_host` (Optional): Your LightStep Satellite Pool Hostname. Defaults to `ingest.lightstep.com`.
* `satellite_port` (Optional): Your LightStep Satellite Pool Port. Defaults to `443`.
* `service_name` (Optional): The service name for spans reported by this collector. Defaults to `opentelemetry-collector`. 
* `plain_text` (Optional): False for HTTPS LightStep Satellite Pools. Defaults to `False`.
Example:

```yaml
exporters:
    lightstep:
        access_token: "abcdef12345"
        satellite_host: "my.satellite.pool.coolcat.com"
        satellite_port: 8000
        service_name: "myService"
        plain_text: true
```
