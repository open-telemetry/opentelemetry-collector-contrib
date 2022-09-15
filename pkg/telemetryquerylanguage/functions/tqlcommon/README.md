# Common Functions

The following functions can be used in any implementation of the Telemetry Query Language.  Although they are tested using [pdata](https://github.com/open-telemetry/opentelemetry-collector/tree/main/pdata) for convenience, the function implementation only interact with native Go types or types defined in the [tql package](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/telemetryquerylanguage/tql).

Factory Functions
- [Join](#join)
- [IsMatch](#ismatch)
- [Int](#int)

Functions
- [set](#set)
- [replace_match](#replace_match)
- [replace_pattern](#replace_pattern)

## Concat

`Concat(delimiter, ...values)`

The `Concat` factory function takes a delimiter and a sequence of values and concatenates their string representation. Unsupported values, such as lists or maps that may substantially increase payload size, are not added to the resulting string.

`delimiter` is a string value that is placed between strings during concatenation. If no delimiter is desired, then simply pass an empty string.

`values` is a series of values passed as arguments. It supports paths, primitive values, and byte slices (such as trace IDs or span IDs).

Examples:

- `Concat(": ", attributes["http.method"], attributes["http.path"])`

- `Concat(" ", name, 1)`

- `Concat("", "HTTP method is: ", attributes["http.method"])`

## IsMatch

`IsMatch(target, pattern)`

The `IsMatch` factory function returns true if the `target` matches the regex `pattern`.

`target` is either a path expression to a telemetry field to retrieve or a literal string. `pattern` is a regexp pattern.

The function matches the target against the pattern, returning true if the match is successful and false otherwise. If target is nil or not a string false is always returned.

Examples:

- `IsMatch(attributes["http.path"], "foo")`


- `IsMatch("string", ".*ring")`

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

## set

`set(target, value)`

The `set` function allows users to set a telemetry field using a value.

`target` is a path expression to a telemetry field. `value` is any value type. If `value` resolves to `nil`, e.g. it references an unset map value, there will be no action.

How the underlying telemetry field is updated is decided by the path expression implementation provided by the user to the `tql.ParseQueries`.

Examples:

- `set(attributes["http.path"], "/foo")`


- `set(name, attributes["http.route"])`


- `set(trace_state["svc"], "example")`


- `set(attributes["source"], trace_state["source"])`

## replace_match

`replace_match(target, pattern, replacement)`

The `replace_match` function allows replacing entire strings if they match a glob pattern.

`target` is a path expression to a telemetry field. `pattern` is a string following [filepath.Match syntax](https://pkg.go.dev/path/filepath#Match). `replacement` is a string. 

If `target` matches `pattern` it will get replaced with `replacement`.

Examples:

- `replace_match(attributes["http.target"], "/user/*/list/*", "/user/{userId}/list/{listId}")`

## replace_pattern

`replace_pattern(target, regex, replacement)`

The `replace_pattern` function allows replacing all string sections that match a regex pattern with a new value.

`target` is a path expression to a telemetry field. `regex` is a regex string indicating a segment to replace. `replacement` is a string.

If one or more sections of `target` match `regex` they will get replaced with `replacement`.

Examples:

- `replace_pattern(resource.attributes["process.command_line"], "password\\=[^\\s]*(\\s?)", "password=***")`