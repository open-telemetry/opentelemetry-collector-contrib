# Resource Context

The Resource Context is a Context implementation for [pdata Resources](https://github.com/open-telemetry/opentelemetry-collector/blob/main/pdata/pcommon/generated_resource.go), the Collector's internal representation for an OTLP Resource.  This Context should be used when interacting only with OTLP resources.

## Paths
In general, the Resource Context supports accessing pdata using the field names from the [resource proto](https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/resource/v1/resource.proto).  All integers are returned and set via `int64`.  All doubles are returned and set via `float64`.

## Enums

The Resource Context does not define any Enums at this time.
