# Profile Context

The Profile Context is a Context implementation for [pdata Profiles](https://github.com/open-telemetry/opentelemetry-collector/tree/main/pdata/pprofile), the collector's internal representation for OTLP profile data.  This Context should be used when interacted with OTLP profiles.

## Paths
In general, the Profile Context supports accessing pdata using the field names from the [profiles proto](https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/profiles/v1development/profiles.proto).  All integers are returned and set via `int64`.  All doubles are returned and set via `float64`.

The following paths are supported.

| path                                   | field accessed                                                                                                                                     | type                                                                    |
|----------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------|
| cache                                  | the value of the current transform context's temporary cache. cache can be used as a temporary placeholder for data during complex transformations | pcommon.Map                                                             |
| cache\[""\]                            | the value of an item in cache. Supports multiple indexes to access nested fields.                                                                  | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| resource                               | resource of the profile being processed                                                                                                            | pcommon.Resource                                                        |
| resource.attributes                    | resource attributes of the profile being processed                                                                                                 | pcommon.Map                                                             |
| resource.attributes\[""\]              | the value of the resource attribute of the profile being processed. Supports multiple indexes to access nested fields.                             | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| instrumentation_scope                  | instrumentation scope of the profile being processed                                                                                               | pcommon.InstrumentationScope                                            |
| instrumentation_scope.name             | name of the instrumentation scope of the profile being processed                                                                                   | string                                                                  |
| instrumentation_scope.version          | version of the instrumentation scope of the profile being processed                                                                                | string                                                                  |
| instrumentation_scope.attributes       | instrumentation scope attributes of the data point being processed                                                                                 | pcommon.Map                                                             |
| instrumentation_scope.attributes\[""\] | the value of the instrumentation scope attribute of the data point being processed. Supports multiple indexes to access nested fields.             | string, bool, int64, float64, pcommon.Map, pcommon.Slice, []byte or nil |
| sample_type                            | the sample types of the profile being processed                                                                                                    | pprofile.ValueTypeSlice                                                 |
| sample                                 | the samples of the profile being processed                                                                                                         | pprofile.SampleSlice                                                    |
| mapping_table                          | the mapping table of the profile being processed                                                                                                   | pprofile.MappingSlice                                                   |
| location_table                         | the location table of the profile being processed                                                                                                  | pprofile.LocationSlice                                                  |
| location_indices                       | the location indices of the profile being processed                                                                                                | pcommon.Int32Slice                                                      |
| function_table                         | the function table of the profile being processed                                                                                                  | pprofile.FunctionSlice                                                  |
| attribute_table                        | the attribute table of the profile being processed                                                                                                 | pprofile.AttributeTableSlice                                            |
| attribute_units                        | the attribute units of the profile being processed                                                                                                 | pprofile.AttributeUnitSlice                                             |
| link_table                             | the link table of the profile being processed                                                                                                      | pprofile.LinkSlice                                                      |
| string_table                           | the string table of the profile being processed                                                                                                    | pcommon.StringSlice                                                     |
| time_unix_nano                         | the time in unix nano of the profile being processed                                                                                               | int64                                                                   |
| time                                   | the time of the profile being processed                                                                                                            | time.Time                                                               |
| duration                               | the duration in nanoseconds of the profile being processed                                                                                         | pcommon.Timestamp                                                       |
| period_type                            | the period type of the profile being processed                                                                                                     | pprofile.ValueType                                                      |
| period                                 | the period of the profile being processed                                                                                                          | int64                                                                   |
| comment_string_indices                 | the comment string indices of the profile being processed                                                                                          | pcommon.Int32Slice                                                      |
| default_sample_type_string_index       | the default sample type string index of the profile being processed                                                                                | int32                                                                   |
| profile_id                             | the profile id of the profile being processed                                                                                                      | pprofile.ProfileID                                                      |
| attribute_indices                      | the attribute indices of the profile being processed                                                                                               | pcommon.Int32Slice                                                      |
| dropped_attributes_count               | the dropped attributes count of the profile being processed                                                                                        | uint32                                                                  |
| original_payload_format                | the original payload format of the profile being processed                                                                                         | string                                                                  |
| original_payload                       | the original payload of the profile being processed                                                                                                | pcommon.ByteSlice                                                       |

## Enums

The Profile Context supports the enum names from the [profiles proto](https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/profiles/v1development/profiles.proto).

| Enum Symbol                         | Value |
|-------------------------------------|-------|
| AGGREGATION_TEMPORALITY_UNSPECIFIED | 0     |
| AGGREGATION_TEMPORALITY_DELTA       | 1     |
| AGGREGATION_TEMPORALITY_CUMULATIVE  | 2     |
