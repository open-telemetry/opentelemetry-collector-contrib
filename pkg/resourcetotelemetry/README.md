# Resource to Telemetry

This is an exporter helper for converting resource attributes to telemetry attributes.
This helper can be used to wrap other exporters.

> :warning: This exporter helper should not be added to a service pipeline.

## Configuration

The helper settings can be configured under `resource_constant_labels` (recommended) or `resource_to_telemetry_conversion` (deprecated):

- `resource_constant_labels`:
    - `included`: List of wildcard patterns (e.g., `service*`, `k8s.pod.*`) matching resource attribute keys to include. Note: if `included` is empty and `excluded` is non-empty, all resource attributes except those matched by `excluded` will be included.
    - `excluded`: List of wildcard patterns matching resource attribute keys to exclude, overriding any matches in `included`. If `included` is empty, setting `excluded` implies including all non-excluded attributes.
- `resource_to_telemetry_conversion` (deprecated):
    - `enabled` (default = false) [Deprecated: use `resource_constant_labels.included` instead]: If `enabled` is `true`, all the resource attributes will be converted to metric labels by default. When using `resource_constant_labels`, the equivalent configuration is setting `included: ["*"]`.
    - `exclude_service_attributes` (default = false) [Deprecated: use `resource_constant_labels.excluded` instead]: If set to `true`, the `service.name`, `service.instance.id` and `service.namespace` resource attributes, which are already converted to `job` and `instance` labels respectively, will be excluded from the final metrics. When using `resource_constant_labels`, the equivalent configuration is adding `service.name`, `service.instance.id`, and `service.namespace` to `excluded`.
