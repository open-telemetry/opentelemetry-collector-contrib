## Status

This library is in the process of being transferred from the [opentelemetry-log-collection](https://github.com/open-telemetry/opentelemetry-log-collection) repository. The code is not yet being used by the collector.

## What is an operator?
An operator is a step in a log processing pipeline. It is used to perform a single action on a log entry, such as reading lines from a file, or parsing JSON from a field. Operators are used in log receivers to interpret logs into OpenTelemetry's log data model.

For instance, a user may read lines from a file using the `file_input` operator. From there, the results of this operation may be sent to a `regex_parser` operator that isolate fields based on a regex pattern. Finally, it is common to convert fields into the log data model's top-level fields, such as timestamp, severity, scope, and trace.


## What operators are available?

Parsers:
- [csv_parser](/docs/operators/csv_parser.md)
- [json_parser](/docs/operators/json_parser.md)
- [regex_parser](/docs/operators/regex_parser.md)
- [syslog_parser](/docs/operators/syslog_parser.md)
- [severity_parser](/docs/operators/severity_parser.md)
- [time_parser](/docs/operators/time_parser.md)
- [trace_parser](/docs/operators/trace_parser.md)
- [uri_parser](/docs/operators/uri_parser.md)

Transformers:
- [add](/docs/operators/add.md)
- [copy](/docs/operators/copy.md)
- [filter](/docs/operators/filter.md)
- [flatten](/docs/operators/flatten.md)
- [metadata](/docs/operators/metadata.md)
- [move](/docs/operators/move.md)
- [recombine](/docs/operators/recombine.md)
- [remove](/docs/operators/remove.md)
- [retain](/docs/operators/retain.md)
- [router](/docs/operators/router.md)

Outputs (Useful for debugging):
- [file_output](docs/operators/file_output.md)
- [stdout](/docs/operators/stdout.md)
