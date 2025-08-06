# Profile Sample Context

> [!NOTE]
> This documentation applies only to version `0.132.0` and later. Information on earlier versions is not available.

The Profile Sample Context is a Context implementation for [pdata Profiles](https://github.com/open-telemetry/opentelemetry-collector/tree/main/pdata/pprofile), the collector's internal representation for OTLP profile data. This Context should be used when interacted with OTLP profiles.

## Paths
In general, the Profile Sample Context supports accessing pdata using the field names from the [profiles proto](https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/profiles/v1development/profiles.proto).  All integers are returned and set via `int64`. All doubles are returned and set via `float64`.

The following paths are supported.

| path                                     | field accessed                                                                                                                                     | type                                                                    |
|------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------|
| cache                                    | the value of the current transform context's temporary cache. cache can be used as a temporary placeholder for data during complex transformations | pcommon.Map                                                             |
| cache\[""\]                              | the value of an item in cache. Supports multiple indexes to access nested fields.                                                                  | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| resource                                 | resource of the profile being processed                                                                                                            | pcommon.Resource                                                        |
| resource.attributes                      | resource attributes of the profile being processed                                                                                                 | pcommon.Map                                                             |
| resource.attributes\[""\]                | the value of the resource attribute of the profile being processed. Supports multiple indexes to access nested fields.                             | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| instrumentation_scope                    | instrumentation scope of the profile being processed                                                                                               | pcommon.InstrumentationScope                                            |
| instrumentation_scope.name               | name of the instrumentation scope of the profile being processed                                                                                   | string                                                                  |
| instrumentation_scope.version            | version of the instrumentation scope of the profile being processed                                                                                | string                                                                  |
| instrumentation_scope.attributes         | instrumentation scope attributes of the data point being processed                                                                                 | pcommon.Map                                                             |
| instrumentation_scope.attributes\[""\]   | the value of the instrumentation scope attribute of the data point being processed. Supports multiple indexes to access nested fields.             | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| profilesample.attributes                 | attributes of the profile being processed                                                                                                          | pcommon.Map                                                             |
| profilesample.attributes\[""\]           | the value of the attribute of the profile being processed. Supports multiple indexes to access nested fields.                                      | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| profilesample.locations_start_index      | the locations start index into the ProfilesDictionary of the sample being processed.                                                               | int64                                                                   |
| profilesample.locations_length           | the locations length of the sample being processed.                                                                                                | int64                                                                   |
| profilesample.values                     | the values of the sample being processed.                                                                                                          | []int64                                                                 |
| profilesample.link_index                 | the link index into the ProfilesDictionary of the sample being processed.                                                                          | int64                                                                   |
| profilesample.timestamps_unix_nano     | the timestamps in unix nano associated with the sample being processed.                                                                            | []int64                                                                 |
| profilesample.timestamps               | the timestamps in `time.Time` associated with the sample being processed.                                                                          | []time.Time                                                             |
| profilesample.attribute_indices          | the attribute indices of the sample being processed.                                                                                               | []int64                                                                 |
