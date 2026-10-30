# Span Link Context

The Span Link Context is a Context implementation for [pdata SpanLinks](https://github.com/open-telemetry/opentelemetry-collector/blob/main/pdata/ptrace/generated_spanlink.go), the Collector's internal representation for OTLP Span Link data.  This Context should be used when interacting with individual OTLP Span Links.

## Paths
In general, the Span Link Context supports accessing pdata using the field names from the [traces proto](https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/trace/v1/trace.proto).  All integers are returned and set via `int64`.  All doubles are returned and set via `float64`.

The following paths are supported.

Setting a path to `nil` is handled based on the path's type: `pcommon.Map` and `pcommon.Slice` fields are set to empty, `pcommon.Value` fields become an empty value, and scalar and struct-typed fields return an error.

| path                                   | field accessed                                                                                                                                                                | type                                                                    |
|-----------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------|
| spanlink.cache                          | the value of the current transform context's temporary cache. cache can be used as a temporary placeholder for data during complex transformations                            | pcommon.Map                                                             |
| spanlink.cache\[""\]                    | the value of an item in cache. Supports multiple indexes to access nested fields.                                                                                             | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| resource                                | resource of the span link being processed                                                                                                                                     | pcommon.Resource                                                        |
| resource.attributes                     | resource attributes of the span link being processed                                                                                                                          | pcommon.Map                                                             |
| resource.attributes\[""\]               | the value of the resource attribute of the span link being processed. Supports multiple indexes to access nested fields.                                                      | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| resource.dropped_attributes_count       | number of dropped attributes of the resource of the span link being processed                                                                                                 | int64                                                                   |
| instrumentation_scope                   | instrumentation scope of the span link being processed                                                                                                                        | pcommon.InstrumentationScope                                            |
| instrumentation_scope.name              | name of the instrumentation scope of the span link being processed                                                                                                            | string                                                                  |
| instrumentation_scope.version           | version of the instrumentation scope of the span link being processed                                                                                                         | string                                                                  |
| instrumentation_scope.dropped_attributes_count | number of dropped attributes of the instrumentation scope of the span link being processed                                                                             | int64                                                                   |
| instrumentation_scope.attributes        | instrumentation scope attributes of the span link being processed                                                                                                             | pcommon.Map                                                             |
| instrumentation_scope.attributes\[""\]  | the value of the instrumentation scope attribute of the span link being processed. Supports multiple indexes to access nested fields.                                         | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| span                                    | span of the span link being processed                                                                                                                                         | ptrace.Span                                                             |
| span.*                                  | All fields exposed by the [ottlspan context](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/contexts/ottlspan) can accessed via `span.` | varies                                                                  |
| spanlink.trace_id                       | a byte slice representation of the trace id of the span link being processed                                                                                                  | pcommon.TraceID                                                         |
| spanlink.trace_id.string                | a string representation of the trace id of the span link being processed                                                                                                      | string                                                                  |
| spanlink.span_id                        | a byte slice representation of the span id of the span link being processed                                                                                                   | pcommon.SpanID                                                          |
| spanlink.span_id.string                 | a string representation of the span id of the span link being processed                                                                                                       | string                                                                  |
| spanlink.trace_state                    | the W3C trace state of the span link being processed                                                                                                                          | string                                                                  |
| spanlink.attributes                     | attributes of the span link being processed                                                                                                                                   | pcommon.Map                                                             |
| spanlink.attributes\[""\]               | the value of the attribute of the span link being processed. Supports multiple indexes to access nested fields.                                                               | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| spanlink.dropped_attributes_count       | dropped_attributes_count of the span link being processed                                                                                                                      | int64                                                                   |
| spanlink.flags                          | flags of the span link being processed                                                                                                                                        | int64                                                                   |
| otelcol.*                               | All paths exposed by the [ottlotelcol](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/contexts/ottlotelcol) context.                    | varies                                                                  |

## Enums

The Span Link Context supports the enum names from the [traces proto](https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/trace/v1/trace.proto), inherited via the `span.*` paths above. `SpanLink` itself has no enum-typed fields.

| Enum Symbol           | Value |
|------------------------|-------|
| SPAN_KIND_UNSPECIFIED | 0     |
| SPAN_KIND_INTERNAL    | 1     |
| SPAN_KIND_SERVER      | 2     |
| SPAN_KIND_CLIENT      | 3     |
| SPAN_KIND_PRODUCER    | 4     |
| SPAN_KIND_CONSUMER    | 5     |
| STATUS_CODE_UNSET     | 0     |
| STATUS_CODE_OK        | 1     |
| STATUS_CODE_ERROR     | 2     |