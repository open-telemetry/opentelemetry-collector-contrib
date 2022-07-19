# Traces Context

The Traces Context is a Context implementation for [pdata Traces](https://github.com/open-telemetry/opentelemetry-collector/tree/main/pdata/ptrace), the collector's internal representation for OTLP trace data.  This Context should be used when interacted with OTLP traces.

## Paths
In general, the Traces Context supports accessing pdata using the field names from the [traces proto](https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/trace/v1/trace.proto).  All integers are returned and set via `int64`.  All doubles are returned and set via `float64`.

The following fields are the exception.

| path                          | field accessed                                                         | type                                                                    |
|-------------------------------|------------------------------------------------------------------------|-------------------------------------------------------------------------|
| resource                      | resource of the data point being processed                             | pcommon.Resource                                                        |
| resource.attributes           | resource attributes of the data point being processed                  | pcommon.Map                                                             |
| resource.attributes\[""\]     | the value of the resource attribute of the data point being processed  | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| instrumentation_scope         | instrumentation scope of the data point being processed                | pcommon.InstrumentationScope                                            |
| instrumentation_scope.name    | name of the instrumentation scope of the data point being processed    | string                                                                  |
| instrumentation_scope.version | version of the instrumentation scope of the data point being processed | string                                                                  |
| attributes                    | attributes of the data point being processed                           | pcommon.Map                                                             |
| attributes\[""\]              | the value of the attribute of the data point being processed           | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| trace_id.string               | a string representation of the trace id                                | string                                                                  |
| span_id.string                | a string representation of the span id                                 | string                                                                  |
| trace_state\[""\]             | an individual entry in the trace state                                 | string                                                                  |
| status.code                   | the status code of the span being processed                            | int64                                                                   |
| status.message                | the status message of the span being processed                         | string                                                                  |
## Enums

The Traces Context supports the enum names from the traces proto.
