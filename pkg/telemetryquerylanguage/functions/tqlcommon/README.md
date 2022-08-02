# Common Functions

The following functions can be used in any implementation of the Telemetry Query Language.  Although they are tested using [pdata](https://github.com/open-telemetry/opentelemetry-collector/tree/main/pdata) for convenience, the function implementation only interact with native Go types or types defined in the [tql package](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/telemetryquerylanguage/tql).

Factory Functions
- [IsMatch](#ismatch)

Functions
- [set](#set)
- [replace_match](#replace_match)
- [replace_pattern](#replace_pattern)

## IsMatch

`IsMatch(target, pattern)`

The `IsMatch` factory function returns true if the `target` matches the regex `pattern`.

`target` is either a path expression to a telemetry field to retrieve or a literal string. `pattern` is a regexp pattern.

The function matches the target against the pattern, returning true if the match is successful and false otherwise. If target is nil or not a string false is always returned.

Examples:

- `IsMatch(attributes["http.path"], "foo")`


- `IsMatch("string", ".*ring")`

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