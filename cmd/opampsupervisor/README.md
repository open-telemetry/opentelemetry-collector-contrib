# OpAMP Supervisor for the OpenTelemetry Collector

This is an implementation of an OpAMP Supervisor that runs a Collector instance using configuration provided from an OpAMP server. This implementation
is following a design specified [here](./specification/README.md).
The design is still undergoing changes, and as such this implementation may change as well.

## Experimenting with the supervisor

The supervisor is currently undergoing heavy development and is not ready for any serious use. However, if you would like to test it, you can follow the steps below:

1. Download the [opamp-go](https://github.com/open-telemetry/opamp-go) repository, and run the OpAMP example server in the `internal/examples/server` directory.

   ```shell
   git clone git@github.com:open-telemetry/opamp-go.git
   cd opamp-go/internal/examples/server
   go run .
   ```

   Visit [localhost:4321](http://localhost:4321) to verify that the server is running.

2. From the Collector contrib repository root, build the Collector:

   ```shell
   make otelcontribcol
   ```

3. Run the supervisor in the `cmd/opampsupervisor` directory of Collector contrib repository, substituting `<OS>` for your operating system (`darwin` for macOS, `linux` or `windows`):

   ```shell
   cd cmd/opampsupervisor
   go run . --config examples/supervisor_<OS>.yaml
   ```

4. The supervisor should connect to the OpAMP server and start a Collector instance.

## Persistent data storage
The supervisor persists some data to disk in order to maintain state between restarts. The directory where this data is stored may be specified via the supervisor configuration:
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

📅: Planned, but no issue to track implementation

### OpAMP capabilities

| OpAMP capability               | Status                                                                           |
|--------------------------------|----------------------------------------------------------------------------------|
| AcceptsRemoteConfig            | ✅                                                                               |
| ReportsEffectiveConfig         | ⚠️                                                                               |
| AcceptsPackages                | <https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/34734> |
| ReportsPackageStatuses         | 📅                                                                               |
| ReportsOwnTraces               | 📅                                                                               |
| ReportsOwnMetrics              | ⚠️                                                                               |
| ReportsOwnLogs                 | 📅                                                                               |
| AcceptsOpAMPConnectionSettings | <https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21043> |
| AcceptsOtherConnectionSettings | <https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21043> |
| AcceptsRestartCommand          | <https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21077> |
| ReportsHealth                  | ⚠️                                                                               |
| ReportsRemoteConfig            | <https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21079> |

### Supervisor specification features

| Feature                                                            | Status                                                                           |
|--------------------------------------------------------------------|----------------------------------------------------------------------------------|
| Offers Supervisor configuration including configuring capabilities | ✅                                                                               |
| Starts and stops a Collector using remote configuration            | ⚠️                                                                               |
| Communicates with OpAMP extension running in the Collector         | <https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21071> |
| Updates the Collector binary                                       | 📅                                                                               |
| Configures the Collector to report its own metrics over OTLP       | 📅                                                                               |
| Configures the Collector to report its own logs over OTLP          | 📅                                                                               |
| Sanitization or restriction of Collector config                    | <https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/24310> |
