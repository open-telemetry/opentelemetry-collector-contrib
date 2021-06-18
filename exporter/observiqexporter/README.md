# observIQ Exporter

This exporter supports sending log data to [observIQ](https://observiq.com/)

**This exporter is currently being developed and is not yet ready for use**

## Configuration

The following configuration options are required:

- `api_key` (no default): The API key used to authenticate with observIQ. See the [docs for the settings page](https://docs.observiq.com/docs/overview#settings-page) to generate one.

The following configuration options can also be configured:

- `endpoint` (default `https://nozzle.app.observiq.com/v1/add`): Endpoint where logs are sent to over http(s).
- `agent_id` (default `otel-collector`): ID for identifying the collector deployment. Does not need to be unique, but can be useful for identifying where logs are coming from.
- `timeout` (default `10s`): Http timeout when sending data.
- `insecure_skip_verify` (default: `false`): Whether to skip checking the certificate of the endpoint when sending data over HTTPS.
- `ca_file` (no default) Path to the CA cert to verify the server being connected to.
- `cert_file` (no default) Path to the TLS cert to use for client connections when TLS client auth is required.
- `key_file` (no default) Path to the TLS key to use for TLS required connections.

In addition, this exporter offers queued retry which is enabled by default.
Information about queued retry configuration parameters can be found
[here](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/exporterhelper/README.md).

The full list of settings exposed for this exporter are documented [here](config.go)

This exporter also offers proxy support as documented
[here](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter#proxy-support).