## `time_parser` operator

The `time_parser` operator sets the timestamp on an entry by parsing a value from the body.

### Configuration Fields

| Field                  | Default          | Description |
| ---                    | ---              | ---         |
| `id`                   | `time_parser`    | A unique identifier for the operator. |
| `output`               | Next in pipeline | The connected operator(s) that will receive all outbound entries. |
| `parse_from`           | required         | The [field](../types/field.md) from which the value will be parsed. |
| `layout_type`          | `strptime`       | The type of timestamp. Valid values are `strptime`, `gotime`, and `epoch`. |
| `layout`               | required         | The exact layout of the timestamp to be parsed. |
| `location`             | `Local`          | The geographic location (timezone) to use when parsing a timestamp that does not include a timezone. The available locations depend on the local IANA Time Zone database. [This page](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones) contains many examples, such as `America/New_York`. |
| `time_zone_locations`  |                  | An optional map of timezone abbreviations to IANA location names (e.g. `PDT: America/Los_Angeles`). When the `%Z` directive is used and the parsed abbreviation matches a key in this map, the corresponding IANA location is used for parsing instead of `location`. This allows correct parsing of log streams that contain timestamps from multiple timezones. |
| `if`                   |                  | An [expression](../types/expression.md) that, when set, will be evaluated to determine whether this operator should be used for the given entry. This allows you to do easy conditional parsing without branching logic with routers. |
| `on_error`             | `send`           | The behavior of the operator if it encounters an error. See [on_error](../types/on_error.md). |


### Example Configurations

Several detailed examples are available [here](../types/timestamp.md).
