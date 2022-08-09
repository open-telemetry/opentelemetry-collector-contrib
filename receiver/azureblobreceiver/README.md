# Azure Blob Receiver

| Status                   |             |
| ------------------------ | ----------- |
| Stability                |[development]|
| Supported pipeline types | logs,traces |
| Distributions            | [contrib]   |


This receiver reads logs and trace data from [Azure Blob Storage](https://azure.microsoft.com/services/storage/blobs/).

## Configuration

The following settings are required:

- `connection_string:` (no default): Azure Blob Storage connection key, which can be found in the Azure Blob Storage resource on the Azure Portal.
- `event_hub:`
  `  endpoint:` (no default): Azure Event Hub endpoint triggering on the `Blob Create` event 

The following settings can be optionally configured:

- `logs:`
  `  container_name:` (default = "logs"): Name of the blob container with the logs
- `traces:`
  `  container_name:` (default = "traces"): Name of the blob container with the traces

Example:

```yaml
receivers:
  azureblob:
    connection_string: DefaultEndpointsProtocol=https;AccountName=accountName;AccountKey=+idLkHYcL0MUWIKYHm2j4Q==;EndpointSuffix=core.windows.net
    event_hub:
      endpoint: Endpoint=sb://oteldata.servicebus.windows.net/;SharedAccessKeyName=otelhubbpollicy;SharedAccessKey=mPJVubIK5dJ6mLfZo1ucsdkLysLSQ6N7kddvsIcmoEs=;EntityPath=otellhub    
```

The receiver subscribes [on the events](https://docs.microsoft.com/en-us/azure/storage/blobs/storage-blob-event-overview) published by Azure Blob Storage and handled by Azure Event Hub. When it receives `Blob Create` event, it reads the logs or traces from a corresponding blob and deletes it after processing.




