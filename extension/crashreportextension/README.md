# Crash Report

| Status                   |               |
|--------------------------|---------------|
| Stability                | [development] |
| Distributions            | [contrib]     |


This extension implements a crash report handler that captures panics and attempts to report them to a remote URL.

## Configuration

- `endpoint`: The endpoint where to send crash reports. Required.


```yaml
extensions:
  crashreport:
    endpoint: "https://example.com/crashreport"

service:
  extensions: [crashreport]
```


[development]:https://github.com/open-telemetry/opentelemetry-collector#development
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
