# Azure Blob Exporter

This exporter saves logs and trace data to [Azure Blob Storage](https://azure.microsoft.com/services/storage/blobs/).

## Configuration

The following settings are required:

- `connection_string` (no default): Azure Blob Storage connection key, which can be found in the Azure Blob Storage resource on the Azure Portal.

The following settings can be optionally configured:

- `logs_container_name` (default = "logs"): Name of the container where the exporter saves blobs with logs.
- `traces_container_name` (default = "traces"): Name of the container where the exporter saves blobs with traces.

Example:

```yaml
exporters:
  azureblob:
    connection_string: DefaultEndpointsProtocol=https;AccountName=accountName;AccountKey=+idLkHYcL0MUWIKYHm2j4Q==;EndpointSuffix=core.windows.net
```

