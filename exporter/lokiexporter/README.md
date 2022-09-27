# Loki Exporter

| Status                   |           |
| ------------------------ |-----------|
| Stability                | [beta]    |
| Supported pipeline types | logs      |
| Distributions            | [contrib] |

Exports data via HTTP to [Loki](https://grafana.com/docs/loki/latest/).

## Getting Started

The following settings are required:

- `endpoint` (no default): The target URL to send Loki log streams to (e.g.: `http://loki:3100/loki/api/v1/push`).

The following options are now deprecated:

- `labels.{attributes/resource}`. Deprecated and will be removed by v0.59.0. See the [Labels](#labels) section for more information.
- `labels.record`. Deprecated and will be removed by v0.59.0. See the [Labels](#labels) section for more information.
- `tenant`: Deprecated and will be removed by v0.59.0. See the [Labels](#tenant-information) section for more information.
- `format` Deprecated without replacement. If you rely on this, let us know by opening an issue before v0.59.0 and we'll assist you in finding a solution.

Example:
```yaml
receivers:
  otlp:

exporters:
  loki:
    endpoint: https://loki.example.com:3100/loki/api/v1/push

processors:
  attributes:
    actions:
    - action: insert
      key: loki.attribute.labels
      value: http.status_code

  resource:
    attributes:
    - action: insert
      key: loki.attribute.labels
      value: http.status
    - action: insert
      key: loki.resource.labels
      value: host.name, pod.name

extensions:

service:
  extensions:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [resource, attributes]
      exporters: [loki]
```

The full list of settings exposed for this exporter are documented [here](./config.go) with detailed sample
configurations [here](./testdata/config.yaml).

## Labels

The Loki exporter can convert OTLP resource and log attributes into Loki labels, which are indexed. For that, you need to configure
hints, specifying which attributes should be placed as labels. The hints are themselves attributes and will be ignored when
exporting to Loki. The following example uses the `attributes` processor to hint the Loki exporter to set the `http.status_code` 
attribute as label and the `resource` processor to give a hint to the Loki exporter to set the `pod.name` as label.

```yaml
processors:
  attributes:
    actions:
    - action: insert
      key: loki.attribute.labels
      value: http.status_code

  resource:
    attributes:
    - action: insert
      key: loki.resource.labels
      value: pod.name
```

## Tenant information

It is recommended to use the [`header_setter`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/headerssetter) extension to configure the tenant information to send to Loki. In case a static tenant
should be used, you can make use of the `headers` option for regular HTTP client settings, like the following:

```yaml
exporters:
  loki:
    endpoint: http://localhost:3100/loki/api/v1/push
    headers:
      "X-Scope-OrgID": acme
```

## Advanced Configuration

Several helper files are leveraged to provide additional capabilities automatically:

- [HTTP settings](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/confighttp/README.md)
- [Queuing and retry settings](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/exporterhelper/README.md)

[beta]:https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
