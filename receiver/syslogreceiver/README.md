# Syslog Receiver

Parses Syslogs from tcp/udp using the [opentelemetry-log-collection](https://github.com/open-telemetry/opentelemetry-log-collection) library.

Supported pipeline types: logs

> :construction: This receiver is in alpha and configuration fields are subject to change.

## Required Parameters

- `operators` is an array of [operators](https://github.com/open-telemetry/opentelemetry-log-collection/blob/main/docs/operators/README.md#what-operators-are-available). Each operator performs a simple responsibility, such as parsing a timestamp or JSON. Chain together operators to process logs into a desired format.

## Optional Parameters

- `plugin_dir` is the path to a directory which contains `stanza` [plugins](https://github.com/open-telemetry/opentelemetry-log-collection/blob/main/docs/plugins.md#using-plugins). Plugins are parameterized pipelines that are designed for specific use cases.
- `offsets_file` is the path to a file that `stanza` will use to remember where it left off when reading from files or other persistent input sources. If specified, `stanza` will create and manage this file.

## Operator Basics

- Every operator has a `type`.
- Every operator can be given a unique `id`. If you use the same type of operator more than once in a pipeline, you must specify an `id`. Otherwise, the `id` defaults to the value of `type`.
- Operators will output to the next operator in the pipeline. The last operator in the pipeline will emit from the receiver. Optionally, the `output` parameter can be used to specify the `id` of another operator to which logs will be passed directly.

Receiver Configuration
```yaml
receivers:
  syslog:
    tcp:
      listen_address: "0.0.0.0:54526"
    syslog:
      protocol: rfc5424
```
