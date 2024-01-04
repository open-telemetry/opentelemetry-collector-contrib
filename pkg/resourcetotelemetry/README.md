# Resource to Telemetry

This is an exporter helper for converting resource attributes to telemetry attributes.
This helper can be used to wrap other exporters.

> :warning: This exporter helper should not be added to a service pipeline.

## Configuration

The following configuration options can be modified:

- `resource_to_telemetry_conversion`
    - `enabled` (default = false): If `enabled` is `true`, all the resource attributes will be converted to metric labels by default.
    - `clear_after_copy` (default = false): If `clear_after_copy` is `true`, all the resource attributes will be cleared after they've been copied.
