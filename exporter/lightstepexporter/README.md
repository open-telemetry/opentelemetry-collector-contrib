# LightStep Exporter

This exporter supports sending trace data to [LightStep](https://www.lightstep.com)

The following configuration options are supported:

* `access_token` (Required): The access token for your LightStep project.
* `satellite_host` (Optional): Your LightStep Satellite Pool URL. Defaults to `https://ingest.lightstep.com`.
* `satellite_port` (Optional): Your LightStep Satellite Pool Port. Defaults to `443`.
* `service_name` (Optional): The service name for spans reported by this collector. Defaults to `opentelemetry-collector`. 

Example:

```yaml
exporters:
    lightstep:
        access_token: "abcdef12345"
        satellite_host: "https://my.satellite.pool.coolcat.com"
        satellite_port: 8000
        service_name: "myService"
```