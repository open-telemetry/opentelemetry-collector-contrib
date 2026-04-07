# Exemplar Context

The Exemplar Context is a Context implementation for [pdata Exemplars](https://github.com/open-telemetry/opentelemetry-collector/tree/main/pdata/pmetric), the collector's internal representation for OTLP metric exemplar data. This Context should be used when interacting with individual OTLP exemplars.

## Paths
In general, the Exemplar Context supports accessing pdata using the field names from the [metrics proto](https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/metrics/v1/metrics.proto). All integers are returned and set via `int64`. All doubles are returned and set via `float64`.

All TraceIDs and SpanIDs are returned as pdata [SpanID](https://github.com/open-telemetry/opentelemetry-collector/blob/main/pdata/pcommon/spanid.go) and [TraceID](https://github.com/open-telemetry/opentelemetry-collector/blob/main/pdata/pcommon/traceid.go) types. Use the [SpanID function](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/ottlfuncs/README.md#spanid) and [TraceID function](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/ottlfuncs/README.md#traceid) when interacting with pdata representations of SpanID and TraceID. When checking for nil, instead check against an empty byte slice (`SpanID(0x0000000000000000)` and `TraceID(0x00000000000000000000000000000000)`).

The following paths are supported.

| path                                           | field accessed                                                                                                                                                                      | type                                                                    |
|------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------|
| exemplar.cache                                 | the value of the current transform context's temporary cache. cache can be used as a temporary placeholder for data during complex transformations                                  | pcommon.Map                                                             |
| exemplar.cache\[""\]                           | the value of an item in cache. Supports multiple indexes to access nested fields.                                                                                                   | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| resource                                       | resource of the exemplar being processed                                                                                                                                            | pcommon.Resource                                                        |
| resource.attributes                            | resource attributes of the exemplar being processed                                                                                                                                 | pcommon.Map                                                             |
| resource.attributes\[""\]                      | the value of the resource attribute of the exemplar being processed. Supports multiple indexes to access nested fields.                                                             | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| resource.dropped_attributes_count              | number of dropped attributes of the resource of the exemplar being processed                                                                                                        | int64                                                                   |
| instrumentation_scope                          | instrumentation scope of the exemplar being processed                                                                                                                               | pcommon.InstrumentationScope                                            |
| instrumentation_scope.name                     | name of the instrumentation scope of the exemplar being processed                                                                                                                   | string                                                                  |
| instrumentation_scope.version                  | version of the instrumentation scope of the exemplar being processed                                                                                                                | string                                                                  |
| instrumentation_scope.dropped_attributes_count | number of dropped attributes of the instrumentation scope of the exemplar being processed                                                                                           | int64                                                                   |
| instrumentation_scope.attributes               | instrumentation scope attributes of the exemplar being processed                                                                                                                    | pcommon.Map                                                             |
| instrumentation_scope.attributes\[""\]         | the value of the instrumentation scope attribute of the exemplar being processed. Supports multiple indexes to access nested fields.                                                | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| metric                                         | the metric to which the exemplar being processed belongs                                                                                                                            | pmetric.Metric                                                          |
| metric.*                                       | All fields exposed by the [ottlmetric context](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/contexts/ottlmetric) can be accessed via `metric.` | varies                                                                 |
| datapoint.*                                    | All fields exposed by the [ottldatapoint context](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/contexts/ottldatapoint) can be accessed via `datapoint.` | varies                                                         |
| exemplar.time_unix_nano                        | the time in unix nano of the exemplar being processed                                                                                                                               | int64                                                                   |
| exemplar.time                                  | the time in `time.Time` of the exemplar being processed                                                                                                                             | `time.Time`                                                             |
| exemplar.double_value                          | the double value of the exemplar being processed                                                                                                                                    | float64                                                                 |
| exemplar.int_value                             | the int value of the exemplar being processed                                                                                                                                       | int64                                                                   |
| exemplar.trace_id                              | a byte slice representation of the trace id of the exemplar being processed                                                                                                         | pcommon.TraceID                                                         |
| exemplar.span_id                               | a byte slice representation of the span id of the exemplar being processed                                                                                                          | pcommon.SpanID                                                          |
| exemplar.filtered_attributes                   | filtered attributes of the exemplar being processed                                                                                                                                 | pcommon.Map                                                             |
| exemplar.filtered_attributes\[""\]             | the value of the filtered attribute of the exemplar being processed. Supports multiple indexes to access nested fields.                                                             | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| otelcol.*                                      | All paths exposed by the [ottlotelcol](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/contexts/ottlotelcol) context.                          | varies                                                                  |

## Enums

The Exemplar Context supports the enum names from the [metrics proto](https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/metrics/v1/metrics.proto).

| Enum Symbol                            | Value |
|----------------------------------------|-------|
| FLAG_NONE                              | 0     |
| FLAG_NO_RECORDED_VALUE                 | 1     |
| AGGREGATION_TEMPORALITY_UNSPECIFIED    | 0     |
| AGGREGATION_TEMPORALITY_DELTA          | 1     |
| AGGREGATION_TEMPORALITY_CUMULATIVE     | 2     |
| METRIC_DATA_TYPE_NONE                  | 0     |
| METRIC_DATA_TYPE_GAUGE                 | 1     |
| METRIC_DATA_TYPE_SUM                   | 2     |
| METRIC_DATA_TYPE_HISTOGRAM             | 3     |
| METRIC_DATA_TYPE_EXPONENTIAL_HISTOGRAM | 4     |
| METRIC_DATA_TYPE_SUMMARY               | 5     |
