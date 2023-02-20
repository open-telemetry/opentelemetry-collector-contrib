# DataPoint Context

The DataPoint Context is a Context implementation for [pdata DataPoints](https://github.com/open-telemetry/opentelemetry-collector/tree/main/pdata/pmetric), the collector's internal representation for OTLP metric data points.  This Context should be used when interacting with individual OTLP data points.

## Paths
In general, the DataPoint Context supports accessing pdata using the field names from the [metrics proto](https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/metrics/v1/metrics.proto).  All integers are returned and set via `int64`.  All doubles are returned and set via `float64`.

The following fields are the exception.

| path                                           | field accessed                                                                                                                                     | type                                                                    |
|------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------|
| cache                                          | the value of the current transform context's temporary cache. cache can be used as a temporary placeholder for data during complex transformations | pcommon.Map                                                             |
| cache\[""\]                                    | the value of an item in cache                                                                                                                      | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| resource                                       | resource of the data point being processed                                                                                                         | pcommon.Resource                                                        |
| resource.attributes                            | resource attributes of the data point being processed                                                                                              | pcommon.Map                                                             |
| resource.attributes\[""\]                      | the value of the resource attribute of the data point being processed                                                                              | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| resource.dropped_attributes_count              | number of dropped attributes of the resource of the data point being processed                                                                     | int64                                                                   |
| instrumentation_scope                          | instrumentation scope of the data point being processed                                                                                            | pcommon.InstrumentationScope                                            |
| instrumentation_scope.name                     | name of the instrumentation scope of the data point being processed                                                                                | string                                                                  |
| instrumentation_scope.version                  | version of the instrumentation scope of the data point being processed                                                                             | string                                                                  |
| instrumentation_scope.dropped_attributes_count | number of dropped attributes of the instrumentation scope of the data point being processed                                                        | int64                                                                   |
| instrumentation_scope.attributes               | instrumentation scope attributes of the data point being processed                                                                                 | pcommon.Map                                                             |
| instrumentation_scope.attributes\[""\]         | the value of the instrumentation scope attribute of the data point being processed                                                                 | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| attributes                                     | attributes of the data point being processed                                                                                                       | pcommon.Map                                                             |
| attributes\[""\]                               | the value of the attribute of the data point being processed                                                                                       | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| metric                                         | the metric to which the data point being processed belongs                                                                                         | pmetric.Metric                                                          |
| metric.name                                    | the name of the metric to which the data point being processed belongs                                                                             | string                                                                  |
| metric.description                             | the description of the metric to which the data point being processed belongs                                                                      | string                                                                  |
| metric.unit                                    | the unit of the metric to which the data point being processed belongs                                                                             | string                                                                  |
| metric.type                                    | the type of the metric to which the data point being processed belongs.  See enums below for integer mapping.                                      | int64                                                                   |
| metric.aggregation_temporality                 | the aggregation temporality of the metric to which the data point being processed belongs                                                          | int64                                                                   |
| metric.is_monotonic                            | the monotonicity of the metric to which the data point being processed belongs                                                                     | bool                                                                    |
| positive                                       | the positive buckets of the data point being processed                                                                                             | pmetric.ExponentialHistogramDataPoint                                   |
| positive.offset                                | the offset of the positive buckets of the data point being processed                                                                               | int64                                                                   |
| positive.bucket_counts                         | the bucket_counts of the positive buckets of the data point being processed                                                                        | uint64                                                                  |
| negative                                       | the negative buckets of the data point being processed                                                                                             | pmetric.ExponentialHistogramDataPoint                                   |
| negative.offset                                | the offset of the negative buckets of the data point being processed                                                                               | int64                                                                   |
| negative.bucket_counts                         | the bucket_counts of the negative buckets of the data point being processed                                                                        | uint64                                                                  |

## Enums

The DataPoint Context supports the enum names from the [metrics proto](https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/metrics/v1/metrics.proto). 

In addition, it also supports an enum for metrics data type, with the numeric value being [defined by pdata](https://github.com/open-telemetry/opentelemetry-collector/blob/main/pdata/pmetric/metrics.go).

| Enum Symbol                            | Value |
|----------------------------------------|-------|
| METRIC_DATA_TYPE_NONE                  | 0     |
| METRIC_DATA_TYPE_GAUGE                 | 1     |
| METRIC_DATA_TYPE_SUM                   | 2     |
| METRIC_DATA_TYPE_HISTOGRAM             | 3     |
| METRIC_DATA_TYPE_EXPONENTIAL_HISTOGRAM | 4     |
| METRIC_DATA_TYPE_SUMMARY               | 5     |