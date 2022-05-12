## What is an operator?
An operator is the most basic unit of log processing. Each operator fulfills a single responsibility, such as reading lines from a file, or parsing JSON from a field. Operators are then chained together in a pipeline to achieve a desired result.

For instance, a user may read lines from a file using the `file_input` operator. From there, the results of this operation may be sent to a `regex_parser` operator that creates fields based on a regex pattern. And then finally, these results may be sent to a `elastic_output` operator that writes each line to Elasticsearch.


## What operators are available?

Inputs:
- [file_input](/docs/operators/file_input.md)
- [generate_input](/docs/operators/generate_input.md)
- [journald_input](/docs/operators/journald_input.md)
- [k8s_event_input](/docs/operators/k8s_event_input.md)
- [stdin](/docs/operators/stdin.md)
- [syslog_input](/docs/operators/syslog_input.md)
- [tcp_input](/docs/operators/tcp_input.md)
- [udp_input](/docs/operators/udp_input.md)
- [windows_eventlog_input](/docs/operators/windows_eventlog_input.md)

Parsers:
- [csv_parser](/docs/operators/csv_parser.md)
- [json_parser](/docs/operators/json_parser.md)
- [regex_parser](/docs/operators/regex_parser.md)
- [syslog_parser](/docs/operators/syslog_parser.md)
- [severity_parser](/docs/operators/severity_parser.md)
- [time_parser](/docs/operators/time_parser.md)
- [trace_parser](/docs/operators/trace_parser.md)
- [uri_parser](/docs/operators/uri_parser.md)

Outputs:
- [file_output](docs/operators/file_output.md)
- [stdout](/docs/operators/stdout.md)

General purpose:
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

Or create your own [plugins](/docs/plugins.md) for a technology-specific use case.
