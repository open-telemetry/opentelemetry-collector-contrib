# Instrumentation Scope Context

> [!NOTE]
> This documentation applies only to version `0.120.0` and later. For information on earlier versions, please refer to the previous [documentation](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/release/0.119.x/pkg/ottl/contexts/ottlscope/README.md).

The Instrumentation Scope Context is a Context implementation for [pdata Instrumentation Scope](https://github.com/open-telemetry/opentelemetry-collector/blob/main/pdata/pcommon/generated_instrumentationscope.go), the Collector's internal representation for OTLP instrumentation scope data.  This Context should be used when interacting only with OTLP instrumentation scope.

## Paths
In general, the Instrumentation Scope Context supports accessing pdata using the field names from the instrumentation section in the [common proto](https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/common/v1/common.proto).  All integers are returned and set via `int64`.  All doubles are returned and set via `float64`.

The following paths are supported.

| path                                 | field accessed                                                                                                                                     | type                                                                    |
|--------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------|
| scope.cache                          | the value of the current transform context's temporary cache. cache can be used as a temporary placeholder for data during complex transformations | pcommon.Map                                                             |
| scope.cache\[""\]                    | the value of an item in cache. Supports multiple indexes to access nested fields.                                                                  | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| resource                             | resource of the instrumentation scope being processed                                                                                              | pcommon.Resource                                                        |
| resource.attributes                  | resource attributes of the instrumentation scope being processed                                                                                   | pcommon.Map                                                             |
| resource.attributes\[""\]            | the value of the resource attribute of the instrumentation scope being processed. Supports multiple indexes to access nested fields.               | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| resource.dropped_attributes_count    | number of dropped attributes of the resource of the instrumentation scope being processed                                                          | int64                                                                   |
| scope.name                           | name of the instrumentation scope of the scope being processed                                                                                     | string                                                                  |
| scope.version                        | version of the instrumentation scope of the scope being processed                                                                                  | string                                                                  |
| scope.dropped_attributes_count       | number of dropped attributes of the instrumentation scope of the scope being processed                                                             | int64                                                                   |
| scope.attributes                     | instrumentation scope attributes of the scope being processed                                                                                      | pcommon.Map                                                             |
| scope.attributes\[""\]               | the value of the instrumentation scope attribute of the scope being processed. Supports multiple indexes to access nested fields.                  | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| context.client.addr                  | the remote address string from the client info                                                                                                     | string                                                                  |
| context.client.metadata              | client metadata from the incoming request context                                                                                                  | pcommon.Map                                                             |
| context.client.metadata\[""\]        | the value of a specific metadata key from the client context. Returns nil if key missing                                                           | pcommon.Slice or nil                                                    |
| context.client.auth.attributes       | map of all auth attributes extracted from client info with values stringified (JSON for non-string)                                                | pcommon.Map                                                             |
| context.client.auth.attributes\[""\] | specific auth attribute value as a string (JSON for non-string values). Missing keys return empty string                                           | string                                                                  |
| context.grpc.metadata                | incoming gRPC metadata from the context (if present)                                                                                               | pcommon.Map                                                             |
| context.grpc.metadata\[""\]          | values slice for a specific incoming gRPC metadata key. Returns nil if key missing                                                                 | pcommon.Slice or nil                                                    |

## Enums

The Instrumentation Scope Context does not define any Enums at this time.
