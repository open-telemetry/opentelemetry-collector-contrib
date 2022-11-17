# Telemetry generator for OpenTelemetry

| Status                   |                 |
| ------------------------ |-----------------|
| Stability                | [development]   |
| Supported signal types   | traces, metrics |

This utility simulates a client generating **traces** and **metrics**, useful for testing and demonstration purposes.

## Installing

To install the latest version run the following command:

```console
$ go install github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen@latest
```

Check the [`go install` reference](https://go.dev/ref/mod#go-install) to install specific versions.


## -> Telemetrygen is in development

```
telemetrygen traces
```

```
telemetrygen metrics
```

[development]: https://github.com/open-telemetry/opentelemetry-collector#development
