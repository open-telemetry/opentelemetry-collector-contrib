# OTTL Functions

The following functions are intended to be used in implementations of the OpenTelemetry Transformation Language that interact with otel data via the collector's internal data model, [pdata](https://github.com/open-telemetry/opentelemetry-collector/tree/main/pdata). These functions may make assumptions about the types of the data returned by Paths.

Factory Functions
- [Concat](#concat)
- [Int](#int)
- [IsMatch](#ismatch)
- [Join](#join)
- [SpanID](#spanid)
- [Split](#split)
- [TraceID](#traceid)

Functions
- [delete_key](#delete_key)
- [delete_matching_keys](#delete_matching_keys)
- [keep_keys](#keep_keys)
- [limit](#limit)
- [replace_all_matches](#replace_all_matches)
- [replace_all_patterns](#replace_all_patterns)
- [replace_match](#replace_match)
- [replace_pattern](#replace_pattern)
- [set](#set)
- [truncate_all](#truncate_all)

## Concat

`Concat(delimiter, ...values)`

The `Concat` factory function takes a delimiter and a sequence of values and concatenates their string representation. Unsupported values, such as lists or maps that may substantially increase payload size, are not added to the resulting string.

`delimiter` is a string value that is placed between strings during concatenation. If no delimiter is desired, then simply pass an empty string.

`values` is a series of values passed as arguments. It supports paths, primitive values, and byte slices (such as trace IDs or span IDs).

Examples:

- `Concat(": ", attributes["http.method"], attributes["http.path"])`


- `Concat(" ", name, 1)`


- `Concat("", "HTTP method is: ", attributes["http.method"])`

## Int

`Int(value)`

The `Int` factory function converts the `value` to int type.

The returned type is int64.

The input `value` types:
* float64. Fraction is discharged (truncation towards zero).
* string. Trying to parse an integer from string if it fails then nil will be returned.
* bool. If `value` is true, then the function will return 1 otherwise 0.
* int64. The function returns the `value` without changes.

If `value` is another type or parsing failed nil is always returned.

The `value` is either a path expression to a telemetry field to retrieve or a literal.

Examples:

- `Int(attributes["http.status_code"])`


- `Int("2.0")`

## IsMatch

`IsMatch(target, pattern)`

The `IsMatch` factory function returns true if the `target` matches the regex `pattern`.

`target` is either a path expression to a telemetry field to retrieve or a literal string. `pattern` is a regexp pattern.

The function matches the target against the pattern, returning true if the match is successful and false otherwise. If target is nil or not a string false is always returned.

Examples:

- `IsMatch(attributes["http.path"], "foo")`


- `IsMatch("string", ".*ring")`

## SpanID

`SpanID(bytes)`

The `SpanID` factory function returns a `pdata.SpanID` struct from the given byte slice.

`bytes` is a byte slice of exactly 8 bytes.

Examples:

- `SpanID(0x0000000000000000)`

## Split

`Split(target, delimiter)`

The `Split` factory function separates a string by the delimiter, and returns an array of substrings.

`target` is a string. `delimiter` is a string.

If the `target` is not a string or does not exist, the `Split` factory function will return `nil`.

Examples:

- ```Split("A|B|C", "|")```

## TraceID

`TraceID(bytes)`

The `TraceID` factory function returns a `pdata.TraceID` struct from the given byte slice.

`bytes` is a byte slice of exactly 16 bytes.

Examples:

- `TraceID(0x00000000000000000000000000000000)`

## delete_key

`delete_key(target, key)`

The `delete_key` function removes a key from a `pdata.Map`

`target` is a path expression to a `pdata.Map` type field. `key` is a string that is a key in the map.

The key will be deleted from the map.

Examples:

- `delete_key(attributes, "http.request.header.authorization")`


- `delete_key(resource.attributes, "http.request.header.authorization")`

## delete_matching_keys

`delete_matching_keys(target, pattern)`

The `delete_matching_keys` function removes all keys from a `pdata.Map` that match a regex pattern.

`target` is a path expression to a `pdata.Map` type field. `pattern` is a regex string.

All keys that match the pattern will be deleted from the map.

Examples:

- `delete_key(attributes, "http.request.header.authorization")`


- `delete_key(resource.attributes, "http.request.header.authorization")`

## keep_keys

`keep_keys(target, keys...)`

The `keep_keys` function removes all keys from the `pdata.Map` that do not match one of the supplied keys.

`target` is a path expression to a `pdata.Map` type field. `keys` is a slice of one or more strings.

The map will be changed to only contain the keys specified by the list of strings.

Examples:

- `keep_keys(attributes, "http.method")`


- `keep_keys(resource.attributes, "http.method", "http.route", "http.url")`

## limit

`limit(target, limit, priority_keys)`

The `limit` function reduces the number of elements in a `pdata.Map` to be no greater than the limit.

`target` is a path expression to a `pdata.Map` type field. `limit` is a non-negative integer.
`priority_keys` is a list of strings of attribute keys that won't be dropped during limiting.

The number of priority keys must be less than the supplied `limit`.

The map will be mutated such that the number of items does not exceed the limit.
The map is not copied or reallocated.

Which items are dropped is random, provide `priority_keys` to preserve required keys.

Examples:

- `limit(attributes, 100)`


- `limit(resource.attributes, 50, "http.host", "http.method")`

## replace_all_matches

`replace_all_matches(target, pattern, replacement)`

The `replace_all_matches` function replaces any matching string value with the replacement string.

`target` is a path expression to a `pdata.Map` type field. `pattern` is a string following [filepath.Match syntax](https://pkg.go.dev/path/filepath#Match). `replacement` is a string.

Each string value in `target` that matches `pattern` will get replaced with `replacement`. Non-string values are ignored.

Examples:

- `replace_all_matches(attributes, "/user/*/list/*", "/user/{userId}/list/{listId}")`

## replace_all_patterns

`replace_all_patterns(target, regex, replacement)`

The `replace_all_patterns` function replaces any segments in a string value that match the regex pattern with the replacement string.

`target` is a path expression to a `pdata.Map` type field. `regex` is a regex string indicating a segment to replace. `replacement` is a string.

If one or more sections of `target` match `regex` they will get replaced with `replacement`.

Examples:

- `replace_all_patterns(attributes, "/account/\\d{4}", "/account/{accountId}")`

## replace_pattern

`replace_pattern(target, regex, replacement)`

The `replace_pattern` function allows replacing all string sections that match a regex pattern with a new value.

`target` is a path expression to a telemetry field. `regex` is a regex string indicating a segment to replace. `replacement` is a string.

If one or more sections of `target` match `regex` they will get replaced with `replacement`.

Examples:

- `replace_pattern(resource.attributes["process.command_line"], "password\\=[^\\s]*(\\s?)", "password=***")`


## replace_match

`replace_match(target, pattern, replacement)`

The `replace_match` function allows replacing entire strings if they match a glob pattern.

`target` is a path expression to a telemetry field. `pattern` is a string following [filepath.Match syntax](https://pkg.go.dev/path/filepath#Match). `replacement` is a string.

If `target` matches `pattern` it will get replaced with `replacement`.

Examples:

- `replace_match(attributes["http.target"], "/user/*/list/*", "/user/{userId}/list/{listId}")`

## set

`set(target, value)`

The `set` function allows users to set a telemetry field using a value.

`target` is a path expression to a telemetry field. `value` is any value type. If `value` resolves to `nil`, e.g. it references an unset map value, there will be no action.

How the underlying telemetry field is updated is decided by the path expression implementation provided by the user to the `ottl.ParseQueries`.

Examples:

- `set(attributes["http.path"], "/foo")`


- `set(name, attributes["http.route"])`


- `set(trace_state["svc"], "example")`


- `set(attributes["source"], trace_state["source"])`

## truncate_all

`truncate_all(target, limit)`

The `truncate_all` function truncates all string values in a `pdata.Map` so that none are longer than the limit.

`target` is a path expression to a `pdata.Map` type field. `limit` is a non-negative integer.

The map will be mutated such that the number of characters in all string values is less than or equal to the limit. Non-string values are ignored.

Examples:

- `truncate_all(attributes, 100)`


- `truncate_all(resource.attributes, 50)`