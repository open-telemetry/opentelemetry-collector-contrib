# Metric Context

The Metric Context is a Context implementation for [pdata Metric](https://github.com/open-telemetry/opentelemetry-collector/tree/main/pdata/pmetric), the collector's internal representation for OTLP metrics.  This Context should be used when interacting with individual OTLP metrics.

## Paths
In general, the Metric Context supports accessing pdata using the field names from the [metrics proto](https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/metrics/v1/metrics.proto).  All integers are returned and set via `int64`.  All doubles are returned and set via `float64`.

The following fields are the exception.

| path                                   | field accessed                                                                 | type                                                                    |
|----------------------------------------|--------------------------------------------------------------------------------|-------------------------------------------------------------------------|
| resource                               | resource of the metric being processed                                         | pcommon.Resource                                                        |
| resource.attributes                    | resource attributes of the metric being processed                              | pcommon.Map                                                             |
| resource.attributes\[""\]              | the value of the resource attribute of the metric being processed              | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| instrumentation_scope                  | instrumentation scope of the metric being processed                            | pcommon.InstrumentationScope                                            |
| instrumentation_scope.name             | name of the instrumentation scope of the metric being processed                | string                                                                  |
| instrumentation_scope.version          | version of the instrumentation scope of the metric being processed             | string                                                                  |
| instrumentation_scope.attributes       | instrumentation scope attributes of the metric being processed                 | pcommon.Map                                                             |
| instrumentation_scope.attributes\[""\] | the value of the instrumentation scope attribute of the metric being processed | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |

## Enums

The Metrics Context supports the enum names from the [metrics proto](https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/metrics/v1/metrics.proto).

In addition, it also supports an enum for metrics data type, with the numeric value being [defined by pdata](https://github.com/open-telemetry/opentelemetry-collector/blob/main/pdata/pmetric/metrics.go).

| Enum Symbol                            | Value |
|----------------------------------------|-------|
| METRIC_DATA_TYPE_NONE                  | 0     |
| METRIC_DATA_TYPE_GAUGE                 | 1     |
| METRIC_DATA_TYPE_SUM                   | 2     |
| METRIC_DATA_TYPE_HISTOGRAM             | 3     |
| METRIC_DATA_TYPE_EXPONENTIAL_HISTOGRAM | 4     |
| METRIC_DATA_TYPE_SUMMARY               | 5     |