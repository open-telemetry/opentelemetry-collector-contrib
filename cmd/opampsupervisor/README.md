# OpAMP Supervisor for the OpenTelemetry Collector

This is an implementation of an OpAMP Supervisor that runs a Collector instance using configuration provided from an OpAMP server. This implementation
is following a design specified [here](./specification/README.md).
The design is still undergoing changes, and as such this implementation may change as well.

Binary and container images for the Supervisor are available under tags starting with `cmd/opampsupervisor` [here](https://github.com/open-telemetry/opentelemetry-collector-releases/tags).

## More information.

If you'd like to learn more about OpAMP, see the
[OpAMP specification](https://github.com/open-telemetry/opamp-spec/blob/main/specification.md#open-agent-management-protocol).
For production-ready client and server implementations in Go, see the
[OpAMP Go repository](https://github.com/open-telemetry/opamp-go).

If you have questions or would like to participate, we'd love to hear from you.
There is a regular SIG meeting, see the
[meeting notes](https://docs.google.com/document/d/19WA5-ex8rNFIBIyVb5VqMXfWNmUQwppGhN8zBeNG0f4)
for more information. You can also get in touch with us in the
[#otel-opamp](https://cloud-native.slack.com/archives/C02J58HR58R) channel on
the CNCF Slack workspace.

## Using the Supervisor

See tags starting with `cmd/opampsupervisor` for binary and container image
builds of the Supervisor
[here](https://github.com/open-telemetry/opentelemetry-collector-releases/tags).

To use the Supervisor, you will need four things:

1. A Supervisor binary, which can be obtained through the link above.
2. A Collector binary that you would like to control through the Supervisor.
3. A Supervisor config file. See examples [here](./examples/).
4. A running OpAMP server.

To test the Supervisor with an example server, download the
[opamp-go](https://github.com/open-telemetry/opamp-go) repository, and run the
OpAMP example server in the `internal/examples/server` directory.

   ```shell
   git clone git@github.com:open-telemetry/opamp-go.git
   cd opamp-go/internal/examples/server
   go run .
   ```

   Visit [localhost:4321](http://localhost:4321) to verify that the server is running.


## Persistent data storage

The supervisor persists some data to disk in order to mantain state between restarts. The directory where this data is stored may be specified via the supervisor configuration:
```yaml
storage:
  directory: "/path/to/storage/dir"
```

By default, the supervisor will use `/var/lib/otelcol/supervisor` on posix systems, and `%ProgramData%/Otelcol/Supervisor` on Windows.

This directory will be created on supervisor startup if it does not exist.

## Status

The OpenTelemetry OpAMP Supervisor is intended to be the reference
implementation of an OpAMP Supervisor, and as such will support all OpAMP
capabilities. Additionally, it follows a design document for the features it
intends to support.

For a list of open issues related to the Supervisor, see [these issues](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues?q=is%3Aopen+is%3Aissue+label%3Acmd%2Fopampsupervisor).

**Key**:

✅: Fully implemented

⚠️: Implemented with caveats

### OpAMP capabilities

| OpAMP capability               | Status                                                                           |
|--------------------------------|----------------------------------------------------------------------------------|
| AcceptsRemoteConfig            | ✅                                                                               |
| ReportsEffectiveConfig         | ✅                                                                               |
| AcceptsPackages                | <https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/34734> |
| ReportsPackageStatuses         | <https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/38727> |
| ReportsOwnTraces               | ✅                                                                               |
| ReportsOwnMetrics              | ✅                                                                               |
| ReportsOwnLogs                 | ✅                                                                               |
| AcceptsOpAMPConnectionSettings | ✅                                                                               |
| AcceptsOtherConnectionSettings | ✅                                                                               |
| AcceptsRestartCommand          | ✅                                                                               |
| ReportsHealth                  | ⚠️                                                                               |
| ReportsStatus                  | <https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/38729> |
| ReportsRemoteConfig            | <https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21079> |
| ReportsAvailableComponents     | ✅                                                                               |

### Supervisor specification features

| Feature                                                            | Status                                                                           |
|--------------------------------------------------------------------|----------------------------------------------------------------------------------|
| Offers Supervisor configuration including configuring capabilities | ✅                                                                               |
| Starts and stops a Collector using remote configuration            | ✅                                                                               |
| Communicates with OpAMP extension running in the Collector         | ✅                                                                               |
| Updates the Collector binary                                       | <https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/33947> |
| Configures the Collector to report it's own metrics over OTLP      | ✅                                                                               |
| Configures the Collector to report it's own logs over OTLP         | ✅                                                                               |
| Sanitization or restriction of Collector config                    | <https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/24310> |
