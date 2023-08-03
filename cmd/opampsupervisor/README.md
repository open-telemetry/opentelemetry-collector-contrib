# OpAMP Supervisor for the OpenTelemetry Collector

This is an implementation of an OpAMP Supervisor that runs a Collector instance using configuration provided from an OpAMP server. This implementation
is following a design specified [here](./specification/README.md).
The design is still undergoing changes, and as such this implementation may change as well.

## Experimenting with the supervisor

The supervisor is currently undergoing heavy development and is not ready for any serious use. However, if you would like to test it, you can follow the steps below:

1. Download the [opamp-go](https://github.com/open-telemetry/opamp-go) repository, and run the OpAMP example server in the `internal/examples/server` directory.
2. From the Collector contrib repository root, build the Collector:

   ```shell
   make otelcontribcol
   ```

3. Run the supervisor, substituting `<OS>` for your platform:

   ```shell
   go run . --config testdata/supervisor_<OS>.yaml
   ```

4. The supervisor should connect to the OpAMP server and start a Collector instance.

## Status

The OpenTelemetry OpAMP Supervisor is intended to be the reference
implementation of an OpAMP Supervisor, and as such will support all OpAMP
capabilities. Additionally, it follows a design document for the features it
intends to support.

For a list of open issues related to the Supervisor, see [these issues](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues?q=is%3Aopen+is%3Aissue+label%3Acmd%2Fopampsupervisor).

### OpAMP capabilities

**Key**:

✅: Fully implemented

⚠️: Implemented with caveats

📅: Planned, but no issue to track implementation

| Status                                                                           | OpAMP capability               |
|----------------------------------------------------------------------------------|--------------------------------|
| ✅                                                                               | AcceptsRemoteConfig            |
| ⚠️                                                                               | ReportsEffectiveConfig         |
| 📅                                                                               | AcceptsPackages                |
| 📅                                                                               | ReportsPackageStatuses         |
| 📅                                                                               | ReportsOwnTraces               |
| ⚠️                                                                               | ReportsOwnMetrics              |
| 📅                                                                               | ReportsOwnLogs                 |
| <https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21043> | AcceptsOpAMPConnectionSettings |
| <https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21043> | AcceptsOtherConnectionSettings |
| <https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21077> | AcceptsRestartCommand          |
| ⚠️                                                                               | ReportsHealth                  |
| <https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21079> | ReportsRemoteConfig            |

### Supervisor specification features

| Status                                                                           | Feature                                                                      |
|----------------------------------------------------------------------------------|------------------------------------------------------------------------------|
| ✅                                                                               | Offers Supervisor configuration including configuring capabilities           |
| ⚠️                                                                               | Starts and stops a Collector using remote configuration                      |
| <https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21071> | Communicates with OpAMP extension running in the Collector                   |
| 📅                                                                               | Updates the Collector binary                                                 |
| 📅                                                                               | Configures the Collector to report it's own metrics over OTLP                |
| 📅                                                                               | Configures the Collector to report it's own logs over OTLP                   |
| <https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/24310> | Sanitization or restriction of Collector config                              |
