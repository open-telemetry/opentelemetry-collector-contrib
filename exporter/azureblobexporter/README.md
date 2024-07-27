# Azure Blob Exporter for OpenTelemetry Collector

## Configuration

The following settings are required:

- url (no default): Azure storage account endpoint. e.g. https://<account-name>.blob.core.windows.net/
- auth (no default): Authentication method for exporter to ingest data.
  - type (no default): Authentication type for expoter. supported values are: connection_string, service_principal, system_managed_identity, user_managed_identity and etc.
  - tenand_id: Tenand Id for the client, only needed when type is service_principal.
  - client_id: Client Id for the auth, only needed when type is service_principal and user_managed_identity.
  - client_secret: Secret for the client, only needed when type is service_principal.
  - connection_string: Connection string to the endpoint. Only needed for connection_string auth type. Once provided, it'll **override** the `url` parameter to the storage account.
- format (no default): `json` or `proto`, which present otel json or otel protobuf format, the extention will be `json` or `pb`.

The following settings can be optionally configured and have default values:

- container: container for metrics, logs and traces.
  - metrics: container to store metrics. default value is `metrics`.
  - logs: container to store logs. default value is `logs`.
  - traces: container to store traces. default value is `traces`.
- blob_name: blob name.
  - metrics: default is `metrics`.
  - traces: default is `traces`.
  - logs: default is `logs`.
- path_format: blob path format. default is `/2006/01/02/<blob_name>_15_04_05_<serial_num>.<format>`. The date format follows constants in Golang, refer [here](https://go.dev/src/time/format.go).

An example configuration is provided as follows:

```yaml
azureblobexporter:
	url: "https://<your-account>.blob.core.windows.net/"
	container:
		logs: "logs"
		metrics: "metrics"
		traces: "traces"
  format: "json"
	auth:
		type: "connection_string"
    connection_string: "DefaultEndpointsProtocol=https;AccountName=<your-acount>;AccountKey=<account-key>;EndpointSuffix=core.windows.net
```
