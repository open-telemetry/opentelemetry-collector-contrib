# Pure Storage FlashArray Receiver

| Status                   |                     |
| ------------------------ |---------------------|
| Stability                | [in-development]    |
| Supported pipeline types | metrics             |
| Distributions            | [contrib]           |

The Pure Storage FlashArray receiver, receives metrics from Pure Storage internal services hosts.

Supported pipeline types: metrics

## Configuration

The following settings are required:
 -  `endpoint` (default: `http://172.0.0.0:9490/metrics/array`): The URL of the scraper selected endpoint

Example:

```yaml
extensions:
  bearertokenauth/array01:
    token: "..."

receivers:
  purefa:
    endpoint: http://172.0.0.1:9490/metrics
    arrays:
    - address: gse-array01
      auth:
        authenticator: bearertokenauth/array01
    settings:
      reload_intervals:
        array: 10s
        host: 13s
        directories: 15s
        pods: 30s
        volumes: 25s
```

The full list of settings exposed for this receiver are documented [here](./config.go)
with detailed sample configurations [here](./testdata/config.yaml).

[in-development]: https://github.com/open-telemetry/opentelemetry-collector#in-development
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
