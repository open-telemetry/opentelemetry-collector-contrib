# Resource to Telemetry

This is an exporter helper for converting resource attributes to telemetry attributes.
This helper can be used to wrap other exporters.

> :warning: This exporter helper should not be added to a service pipeline.

## Configuration

The following configuration options can be modified:

- `resource_to_telemetry_conversion`
    - `enabled` (default = false): If `enabled` is `true`, all the resource attributes will be converted to metric labels by default.
    -
      `exclude`
      - `match_type` `<strict|regexp>`
      - `regexp`  `<regexp config>`
    By setting a filter option you can exclude some attributes during copying. You can use the strict match type or regexp one.

### Examples

Exclude attribute named `foo` if value match exact `bar`:
```yaml
resource_to_telemetry_conversion:
  enabled: true
  exclude:
    match_type: strict
    attributes:
      - key: foo
        value: bar
 ```

Exclude attribute named `foo` regardless of its value:
```yaml
resource_to_telemetry_conversion:
  enabled: true
  exclude:
    match_type: regexp
    regexp:
      cacheenabled: true
      cachemaxnumentries: 1
    attributes:
      - key: foo
      value: .*
 ```

Exclude attribute named `foo` where value begins with bar:
```yaml
resource_to_telemetry_conversion:
  enabled: true
  exclude:
    match_type: regexp
    regexp:
      cacheenabled: true
      cachemaxnumentries: 1
    attributes:
      - key: foo
        value: bar.*
 ```
