## Parsers

Parser operators are used to isolate values from a string. There are two classes of parsers, simple and complex.

### Simple Parsers

Simple parsers perform a very specific operation where the result is assigned to a predetermined field in the log data model.
For example, the [`time_parser`](../operators/time_parser.md) will parse a timestamp and assign it to the log record's `Timestamp` field.

List of simple parsers:
- [`time_parser`](../operators/time_parser.md)
- [`severity_parser`](../operators/severity_parser.md)
- [`trace_parser`](../operators/trace_parser.md)
- [`scope_name_parser`](../operators/scope_name_parser.md)

### Complex Parsers

Complex parsers differ from simple parsers in several ways.
1. Parsing produces multiple key/value pairs, where the values may be strings, ints, arrays, or objects.
2. By default, these key/value pairs will be added to log entry's `attributes` field. In the case of collisions between keys, the newly parsed value will override the existing value. Alternately, the [field](../types/field.md) to which the object is written can be configured using the `parse_to` setting.
3. The configuration may "embed" certain followup operations. Generally, these operations correlate to the simple parsers listed above. Embedded operations are applied immediately after the primary parsing operation, and only if the primary operation was successful. Each embedded operation is executed independently of the others. (e.g. failure to parse a timestamp will not prevent an attempt to parse severity.)

The following examples illustrate how a `json_parser` may embed timestamp and severity parsers.

Consider a simple json log: `{"message":"foo", "ts":"2022-08-10", "sev":"INFO"}`

```yaml
# Standalone json parser
- type: json_parser

# Regex parser with embedded timestamp and severity parsers
- type: json_parser
  timestamp:
    parse_from: attributes.ts
    layout_type: strptime
    layout: '%Y-%m-%d'
  severity:
    parse_from: attributes.sev
```

Note that when configuring embedded operations, it is typically necessary to reference a field that was set by the primary operation. In the above example, the values specified in the `parse_from` fields have taken into account that the `json_parser` will write key/value pairs to `attributes`.

List of complex parsers:
- [`json_parser`](../operators/json_parser.md)
- [`regex_parser`](../operators/regex_parser.md)
- [`csv_parser`](../operators/csv_parser.md)
- [`key_value_parser`](../operators/key_value_parser.md)
- [`uri_parser`](../operators/uri_parser.md)
- [`syslog_parser`](../operators/syslog_parser.md)

List of embeddable operations:
- [`timestamp`](./timestamp.md)
- [`severity`](./severity.md)
- [`trace`](./trace.md)
- [`scope_name`](./scope_name.md)
- `body`: A field that should be assigned to the log body.
