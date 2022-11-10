# Log Context

The Log Context is a Context implementation for [pdata Logs](https://github.com/open-telemetry/opentelemetry-collector/tree/main/pdata/plog), the collector's internal representation for OTLP log data.  This Context should be used when interacted with OTLP logs.

## Paths
In general, the Log Context supports accessing pdata using the field names from the [logs proto](https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/logs/v1/logs.proto).  All integers are returned and set via `int64`.  All doubles are returned and set via `float64`.

All TraceIDs and SpanIDs are returned as pdata [SpanID](https://github.com/open-telemetry/opentelemetry-collector/blob/main/pdata/pcommon/spanid.go) and [TraceID](https://github.com/open-telemetry/opentelemetry-collector/blob/main/pdata/pcommon/traceid.go) types.  Use the [SpanID function](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/ottlfuncs/README.md#spanid) and [TraceID function](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/ottlfuncs/README.md#traceid) when interacting with pdata representations of SpanID and TraceID.  When checking for nil, instead check against an empty byte slice (`SpanID(0x0000000000000000)` and `TraceID(0x00000000000000000000000000000000)`).

The following fields are the exception.

| path                                           | field accessed                                                                       | type                                                                    |
|------------------------------------------------|--------------------------------------------------------------------------------------|-------------------------------------------------------------------------|
| resource                                       | resource of the log being processed                                                  | pcommon.Resource                                                        |
| resource.attributes                            | resource attributes of the log being processed                                       | pcommon.Map                                                             |
| resource.attributes\[""\]                      | the value of the resource attribute of the log being processed                       | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| resource.dropped_attributes_count              | number of dropped attributes of the resource of the log being processed              | int64                                                                   |
| instrumentation_scope                          | instrumentation scope of the log being processed                                     | pcommon.InstrumentationScope                                            |
| instrumentation_scope.name                     | name of the instrumentation scope of the log being processed                         | string                                                                  |
| instrumentation_scope.version                  | version of the instrumentation scope of the log being processed                      | string                                                                  |
| instrumentation_scope.dropped_attributes_count | number of dropped attributes of the instrumentation scope of the log being processed | int64                                                                   |
| instrumentation_scope.attributes               | instrumentation scope attributes of the data point being processed                   | pcommon.Map                                                             |
| instrumentation_scope.attributes\[""\]         | the value of the instrumentation scope attribute of the data point being processed   | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| attributes                                     | attributes of the log being processed                                                | pcommon.Map                                                             |
| attributes\[""\]                               | the value of the attribute of the log being processed                                | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| trace_id                                       | a byte slice representation of the trace id                                          | pcommon.TraceID                                                         |
| trace_id.string                                | a string representation of the trace id                                              | string                                                                  |
| span_id                                        | a byte slice representation of the span id                                           | pcommon.SpanID                                                          |
| span_id.string                                 | a string representation of the span id                                               | string                                                                  |

## Enums

The Log Context supports the enum names from the [logs proto](https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/logs/v1/logs.proto).
