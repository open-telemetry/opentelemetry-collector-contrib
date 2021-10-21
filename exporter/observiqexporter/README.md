# observIQ Exporter

This exporter supports sending log data to [observIQ](https://observiq.com/)

**This exporter is currently being developed and is not yet ready for use**

## Configuration

One of the following configuration options are required:

- `api_key` (no default): The API key used to authenticate with observIQ. See the [your profile on observiq](https://docs.observiq.com/docs/overview#settings-page) to generate one. This is only required if `secret_key` is not specified, and should not be specified if `secret_key` is specified. *This is the preferred key to use if your collector is not managed by observIQ.*
- `secret_key` (no default): The secret key used to authenticate with observIQ. You can find this by [adding an agent](https://app.observiq.com/universal/installation). This is only required if `api_key` is not specified, and should not be specified if `api_key` is specified.

The following configuration options can also be configured:

- `endpoint` (default: `https://nozzle.app.observiq.com/v1/add`): Endpoint where logs are sent to over http(s).
- `agent_id` (default: uuid generated from os.Hostname): ID for identifying the collector deployment. Ideally, this is a unique uuid, for uniquely identifying specific deployments of the agent. By default, this will be a uuid generated from the hostname. If the hostname cannot be determined, it will be `00000000-0000-0000-0000-000000000000`
- `agent_name` (default: os.Hostname): Name for identifying the collector deployment. This is the friendly name of the deployment. Defaults to the hostname; If the hostname cannot be determined, `otel collector` will be used as a fallback.
- `timeout` (default: `10s`): Http timeout when sending data.
- `tls`:
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