## Dotnet Diagnostics Receiver

This receiver provides a capability similar to the
[dotnet-counters](https://docs.microsoft.com/en-us/dotnet/core/diagnostics/dotnet-counters)
tool, which takes a .NET process ID and reads metrics from that process,
providing them to the CLI. Similarly, this receiver reads metrics from a given
.NET process, translating them and providing them to the Collector.

#### .NET Counters Overview

The .NET runtime makes available metrics to interested clients over an IPC
connection, listening for requests and responding with metrics sent at a
specified interval. All .NET processes newer than 3.0 make available both
default metrics (grouped under the name `System.Runtime`) and any custom metrics
generated via the EventCounter
[API](https://docs.microsoft.com/en-us/dotnet/api/system.diagnostics.tracing.eventcounter?view=net-5.0)
.

Once a .NET process is running, a client (such as this receiver) may connect to
it over IPC, which is either a _Unix domain socket_ on Linux/macOS, or a _named
pipe_ on Windows. After connecting, the client sends the process a request,
using a custom binary encoding, indicating both the counters it's interested in
and the collection interval, then waits for data. If the request is successful,
the .NET process sends metrics, also using a custom binary encoding, over the
IPC connection, at the specified interval.

#### Operation

At startup, this recevier looks for a file in `TMPDIR` (or `/tmp` if not set)
corresponding to the given PID and a naming convention. If found, a Unix domain
socket connection is opened, using the file as the endpoint, and a request is
made to the dotnet process for metrics, with the given (in the config)
collection interval and counters.

After that, it listens for metrics arriving from the connection, and sends them
to the next consumer as soon as they arrive. If the connection fails, or an
unexpected value is read, the receiver shuts down.

#### Configuration

This receiver accepts three configuration fields: `collection_interval`,
`pid`, and `counters`.

| Field Name | Description | Example | Default |
| ---------- | ----------- | ------- | ------- |
| `collection_interval` | The interval between metric collection (converted to seconds) | `1m` | `1s`
| `pid` | The process ID of the .NET process from which to collect metrics | `1001` | |
| `counters` | A list of counter groups (sometimes referred to as _providers_ or _event sources_) to request from the .NET process | `["MyCounters"]` | `["System.Runtime", "Microsoft.AspNetCore.Hosting"]` |

Example yaml config:

```yaml
receivers:
  dotnet_diagnostics:
    collection_interval: 10s
    pid: 23860
    counters: [ "MyCounters", "System.Runtime" ]
exporters:
  logging:
    loglevel: info
service:
  pipelines:
    metrics:
      receivers: [ dotnet_diagnostics ]
      exporters: [ logging ]
```

#### Usage With Receiver Creator

It is possible to create a config file for this receiver with a hard-coded
process id, but it is expected that this receiver will often be used with a
receiver creator, and a host observer, to discover .NET processes at runtime.

Example receiver creator config:

```yaml
extensions:
  host_observer:
receivers:
  receiver_creator:
    watch_observers: [ host_observer ]
    receivers:
      dotnet_diagnostics:
        rule: type.hostport && process_name == 'dotnet'
        config:
          pid: "`process_id`"
exporters:
  logging:
    loglevel: info
service:
  extensions: [ host_observer ]
  pipelines:
    metrics:
      receivers: [ receiver_creator ]
      exporters: [ logging ]
```

#### Supported Versions

This receiver is compatible with .NET Core 3.0 and later versions, running on Linux or
macOS. Windows is not yet supported.

#### Current Status

This receiver is _beta_. It has been tested on macOS and Linux with .NET v3.1.

#### External Resources

https://github.com/dotnet/diagnostics/blob/master/documentation/design-docs/ipc-protocol.md

https://github.com/Microsoft/perfview/blob/main/src/TraceEvent/EventPipe/EventPipeFormat.md
