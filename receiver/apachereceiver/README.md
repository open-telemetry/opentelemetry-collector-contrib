# Apache Web Server Receiver

| Status                   |           |
| ------------------------ |-----------|
| Stability                | [beta]    |
| Supported pipeline types | metrics   |
| Distributions            | [contrib] |

This receiver fetches stats from a Apache Web Server instance using the `server-status?auto` endpoint.

## Prerequisites

This receiver supports Apache Web Server version 2.4+

### mod_status module

In order to receive server statistics, you must configure the server's `httpd.conf` file to [enable status support](https://httpd.apache.org/docs/2.4/mod/mod_status.html).


### Configuration

The following settings are required:
- `endpoint` (default: `http://localhost:8080/server-status?auto`): The URL of the httpd status endpoint

The following settings are optional:
- `collection_interval` (default = `10s`): This receiver collects metrics on an interval. This value must be a string readable by Golang's [time.ParseDuration](https://pkg.go.dev/time#ParseDuration). Valid time units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`.

### Example Configuration

```yaml
receivers:
  apache:
    endpoint: "http://localhost:8080/server-status?auto"
```

The full list of settings exposed for this receiver are documented [here](./config.go) with detailed sample configurations [here](./testdata/config.yaml).

## Metrics

Details about the metrics produced by this receiver can be found in [metadata.yaml](./metadata.yaml)

[beta]: https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib

## Feature gate configurations

See the [Collector feature gates](https://github.com/open-telemetry/opentelemetry-collector/blob/main/featuregate/README.md#collector-feature-gates) for an overview of feature gates in the collector.

**BETA**: `receiver.apache.emitServerNameAsResourceAttribute`

The feature gate `receiver.apache.emitServerNameAsResourceAttribute` once enabled starts emitting the metrics with a resource attribute `apache.server.name`. When the feature gate is disabled, the metrics are emitted with a `server_name` metric-level attribute instead.

This is considered a breaking change for existing users of this receiver, and it is recommended to migrate to the new implementation when possible. Any new users planning to adopt this receiver should enable this feature gate to avoid having to migrate any visualisations or alerts.

This feature gate is enabled by default, and eventually the old implementation will be removed. It aims
to give users time to migrate to the new implementation. The target release for the old implementation to be removed
is 0.69.0.

**BETA**: `receiver.apache.emitPortAsResourceAttribute`

The feature gate `receiver.apache.emitPortAsResourceAttribute` once enabled starts emitting the metrics with a resource attribute `apache.server.port`.

This is considered a breaking change for existing users of this receiver, and it is recommended to migrate to the new implementation when possible. Any new users planning to adopt this receiver should enable this feature gate to avoid having to migrate any visualisations or alerts.

This feature gate is enabled by default, and eventually the old implementation will be removed. It aims
to give users time to migrate to the new implementation. The target release for the old implementation to be removed
is 0.69.0.
