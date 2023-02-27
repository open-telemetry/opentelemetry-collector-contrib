# OpAMP Supervisor for the OpenTelemetry Collector

This is an implementation of an OpAMP Supervisor that runs a Collector instance using configuration provided from an OpAMP server. This implementation
is following a design specified in this [Google Doc](https://docs.google.com/document/d/1KtH5atZQUs9Achbce6LiOaJxLbksNJenvgvyKLsJrkc/edit).
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