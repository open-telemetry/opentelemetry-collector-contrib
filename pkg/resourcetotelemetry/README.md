# Resource to Telemetry

This is an exporter helper for converting resource attributes to telemetry attributes.
This helper can be used to wrap other exporters.

> :warning: This exporter helper should not be added to a service pipeline.

## Configuration

The helper settings can be embedded by exporters under a configuration key of their choice (such as `resource_constant_labels` in Prometheus exporters or `resource_to_telemetry_conversion` in AWS EMF exporter):

- `included`: List of wildcard patterns (e.g., `service*`, `k8s.pod.*`) matching resource attribute keys to include. Wildcards `*` and `?` are supported and match any sequence of characters and any single character respectively (they cannot be escaped). Note: if `included` is empty and `excluded` is non-empty, all resource attributes except those matched by `excluded` will be included.
- `excluded`: List of wildcard patterns matching resource attribute keys to exclude, overriding any matches in `included`. If `included` is empty, setting `excluded` implies including all non-excluded attributes.
- `enabled` (default = false) [Deprecated: use `included` instead]: If `enabled` is `true`, all the resource attributes will be converted to metric labels by default. The equivalent configuration is setting `included: ["*"]`.
- `exclude_service_attributes` (default = false) [Deprecated: use `excluded` instead]: If set to `true`, the `service.name`, `service.instance.id` and `service.namespace` resource attributes, which are already converted to `job` and `instance` labels respectively, will be excluded from the final metrics. The equivalent configuration is adding `service.name`, `service.instance.id`, and `service.namespace` to `excluded`.
