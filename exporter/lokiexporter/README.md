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
      key: event_domain
      from_attribute: event.domain
    - action: insert
      key: loki.attribute.labels
      value: event_domain

  resource:
    attributes:
    - action: insert
      key: service_name
      from_attribute: service.name
    - action: insert
      key: service_namespace
      from_attribute: service.namespace
    - action: insert
      key: loki.resource.labels
      value: service_name, service_namespace

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

More information on how to send logs to Grafana Loki using the OpenTelemetry Collector could be found [here](https://grafana.com/docs/opentelemetry/collector/send-logs-to-loki/)
## Labels

The Loki exporter can convert OTLP resource and log attributes into Loki labels, which are indexed. For that, you need to configure
hints, specifying which attributes should be placed as labels. The hints are themselves attributes and will be ignored when
exporting to Loki. The following example uses the `attributes` processor to hint the Loki exporter to set the `event.domain` 
attribute as label and the `resource` processor to give a hint to the Loki exporter to set the `service.name` as label.

```yaml
processors:
  attributes:
    actions:
      - action: insert
        key: event_domain
        from_attribute: event.domain
      - action: insert
        key: loki.attribute.labels
        value: event_domain

  resource:
    attributes:
      - action: insert
        key: service_name
        from_attribute: service.name
      - action: insert
        key: loki.resource.labels
        value: service_name
```

Currently, Loki does not support labels with dots. 
That’s why to add Loki label based on `event.domain` OTLP attribute we need to specify two actions. The first one inserts a new attribute `event_domain` from the OTLP attribute `event.domain`. The second one is a hint for Loki, specifying that the `event_domain` attribute should be placed as a Loki label.
The same approach is applicable to placing Loki labels from resource attribute `service.name`.

Default labels:
- `job=service.namespace/service.name`
- `instance=service.instance.id`
- `exporter=OTLP`

`exporter=OTLP` is always set.

If `service.name` and `service.namespace` are present then `job=service.namespace/service.name` is set

If `service.name` is present and `service.namespace` is not present then `job=service.name` is set

If `service.name` is not present and `service.namespace` is present then `job` label is not set

If `service.instance.id` is present then `instance=service.instance.id` is set

If `service.instance.id` is not present then `instance` label is not set

## Tenant information

It is recommended to use the [`header_setter`](../../extension/headerssetterextension/README.md) extension to configure the tenant information to send to Loki. In case a static tenant
should be used, you can make use of the `headers` option for regular HTTP client settings, like the following:

```yaml
exporters:
  loki:
    endpoint: http://localhost:3100/loki/api/v1/push
    headers:
      "X-Scope-OrgID": acme
```

It is also possible to provide the `loki.tenant` attribute hint that specifies
which resource or log attributes value should be used as a tenant. For example:

```yaml
processors:
  resource:
    attributes:
    - action: insert
      key: loki.tenant
      value: host_name
    - action: insert
      key: host_name
      from_attribute: host.name
```

In this case the value of the `host.name` resource attribute is used to group logs
by tenant and send requests with the `X-Scope-OrgID` header set to relevant tenants.

If the `loki.tenant` hint attribute is present in both resource or log attributes,
then the look-up for a tenant value from resource attributes takes precedence.

## Severity

OpenTelemetry uses `record.severity` to track log levels where loki uses `record.attributes.level` for the same. The exporter automatically maps the two, except if a "level" attribute already exists.

## Advanced Configuration

Several helper files are leveraged to provide additional capabilities automatically:

- [HTTP settings](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/confighttp/README.md)
- [Queuing and retry settings](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/exporterhelper/README.md)

[beta]:https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
