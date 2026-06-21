# Azure Event Hub Exporter

Exports OpenTelemetry logs, metrics, and traces to Azure Event Hubs using OTLP JSON encoding.

## Setup: Service Principal (Client Secret)

### 1. Create an Event Hub namespace and hub

```bash
# Variables — adjust to your environment
RESOURCE_GROUP="my-rg"
LOCATION="eastus"
NAMESPACE="my-otel-ns"         # must be globally unique
EVENTHUB="otel-telemetry"
SP_NAME="otel-collector"

# Namespace
az eventhubs namespace create \
  --resource-group "$RESOURCE_GROUP" \
  --name "$NAMESPACE" \
  --location "$LOCATION" \
  --sku Standard

# Event Hub inside the namespace
az eventhubs eventhub create \
  --resource-group "$RESOURCE_GROUP" \
  --namespace-name "$NAMESPACE" \
  --name "$EVENTHUB" \
  --partition-count 4 \
  --retention-time-in-hours 24
```

### 2. Create an App Registration and client secret

```bash
# Create the app registration (service principal)
APP_ID=$(az ad app create --display-name "$SP_NAME" --query appId -o tsv)
SP_OBJECT_ID=$(az ad sp create --id "$APP_ID" --query id -o tsv)

# Create a client secret (note the value — it is shown only once)
CLIENT_SECRET=$(az ad app credential reset \
  --id "$APP_ID" \
  --append \
  --years 1 \
  --query password -o tsv)

TENANT_ID=$(az account show --query tenantId -o tsv)

echo "CLIENT_ID:     $APP_ID"
echo "TENANT_ID:     $TENANT_ID"
echo "CLIENT_SECRET: $CLIENT_SECRET"
```

### 3. Grant the service principal permission to send events

The minimum required role is **Azure Event Hubs Data Sender** scoped to the Event Hub (or the entire namespace).

```bash
NAMESPACE_ID=$(az eventhubs namespace show \
  --resource-group "$RESOURCE_GROUP" \
  --name "$NAMESPACE" \
  --query id -o tsv)

az role assignment create \
  --assignee "$SP_OBJECT_ID" \
  --role "Azure Event Hubs Data Sender" \
  --scope "$NAMESPACE_ID"
```

> **Tip:** Scope the role to a single Event Hub instead of the namespace if you want least-privilege:
> `--scope "$NAMESPACE_ID/eventhubs/$EVENTHUB"`

### 4. Configure the collector

```yaml
extensions:
  azure_auth:
    service_principal:
      tenant_id: "<TENANT_ID>"
      client_id: "<APP_ID>"
      client_secret: "<CLIENT_SECRET>"
    # Scope required by the Event Hubs data plane
    scopes:
      - "https://eventhubs.azure.net/.default"

exporters:
  azure_event_hub:
    auth: azure_auth
    event_hub:
      # Fully qualified namespace — do NOT include "https://"
      namespace: "<NAMESPACE>.servicebus.windows.net"
      name: "<EVENTHUB>"

    # Partition strategies — each flag is independent and applies only to its signal.
    # All three can be enabled simultaneously on one exporter instance.
    #
    # Logs: pick at most one (the two are mutually exclusive)
    partition_logs_by_resource_attributes: true   # co-locate by resource/service
    # partition_logs_by_trace_id: true            # co-locate with associated traces
    #
    # Metrics: consistent routing per resource
    partition_metrics_by_resource_attributes: true
    #
    # Traces: co-locate all spans of a trace
    partition_traces_by_id: true

    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s

    sending_queue:
      enabled: true
      num_consumers: 4
      queue_size: 1000

service:
  extensions: [azure_auth]
  pipelines:
    logs:
      receivers: [otlp]
      exporters: [azure_event_hub]
    metrics:
      receivers: [otlp]
      exporters: [azure_event_hub]
    traces:
      receivers: [otlp]
      exporters: [azure_event_hub]
```

### 5. Separate hubs per signal (recommended for production)

Event Hubs does not filter by content type, so mixing logs, metrics, and traces in one hub makes consumer-side routing harder. Use three hubs and three exporter instances:

```yaml
exporters:
  azure_event_hub/logs:
    auth: azure_auth
    event_hub:
      namespace: "<NAMESPACE>.servicebus.windows.net"
      name: "otel-logs"
    partition_logs_by_resource_attributes: true

  azure_event_hub/metrics:
    auth: azure_auth
    event_hub:
      namespace: "<NAMESPACE>.servicebus.windows.net"
      name: "otel-metrics"
    partition_metrics_by_resource_attributes: true

  azure_event_hub/traces:
    auth: azure_auth
    event_hub:
      namespace: "<NAMESPACE>.servicebus.windows.net"
      name: "otel-traces"
    partition_traces_by_id: true

service:
  extensions: [azure_auth]
  pipelines:
    logs:
      receivers: [otlp]
      exporters: [azure_event_hub/logs]
    metrics:
      receivers: [otlp]
      exporters: [azure_event_hub/metrics]
    traces:
      receivers: [otlp]
      exporters: [azure_event_hub/traces]
```

---

## Partition strategies

The three signal-level flags are fully independent — enable any combination on a single exporter instance. The only constraint is within logs: `partition_logs_by_resource_attributes` and `partition_logs_by_trace_id` are mutually exclusive (pick one).

| Flag | Signal | Partition key | Use when |
|---|---|---|---|
| `partition_logs_by_resource_attributes` | Logs | Hash of resource attributes | You want all logs from the same service on the same partition |
| `partition_logs_by_trace_id` | Logs | Trace ID hex string | Your logs carry `trace_id` and you want logs co-located with their traces |
| `partition_metrics_by_resource_attributes` | Metrics | Hash of resource attributes | You want all metrics from the same host/service on the same partition |
| `partition_traces_by_id` | Traces | Trace ID hex string | You want all spans of a trace on the same partition |
| *(none set for a signal)* | Any | None (Event Hubs round-robins) | Order does not matter; simplest setup |

> **Note on message size:** Each partition group is sent as a single Event Hub message. If a single resource's data exceeds the Event Hub message size limit (1 MB on Standard, 100 MB on Premium), the export will fail with a clear error. Reduce the upstream batch size in the `sending_queue` or the upstream batcher processor.

### Examples

#### No partitioning — simplest setup

Event Hubs distributes messages across partitions automatically. Use this when consumers do not need ordering guarantees.

```yaml
exporters:
  azure_event_hub:
    connection: "Endpoint=sb://<namespace>.servicebus.windows.net/;SharedAccessKeyName=<key>;SharedAccessKey=<secret>;EntityPath=otel-telemetry"
```

---

#### Partition logs by resource (service-level ordering)

All logs emitted by `service-a` go to the same partition. A consumer reading partition 2 always sees `service-a` logs in emission order.

Two services → two separate Event Hub messages per batch, each pinned to a different partition.

```yaml
exporters:
  azure_event_hub:
    connection: "Endpoint=sb://<namespace>.servicebus.windows.net/;..."
    partition_logs_by_resource_attributes: true
```

```
Incoming batch (2 resources):
  service-a logs  →  partition key: hash({service.name="service-a"})  →  partition 1
  service-b logs  →  partition key: hash({service.name="service-b"})  →  partition 3
```

---

#### Partition logs by trace ID (co-locate logs with traces)

Use this when your log records include `trace_id` (e.g., from a tracing-instrumented service). Log events and trace spans with the same trace ID land on the same partition, making correlated reads from a single partition consumer straightforward.

```yaml
exporters:
  azure_event_hub:
    connection: "Endpoint=sb://<namespace>.servicebus.windows.net/;..."
    partition_logs_by_trace_id: true
    partition_traces_by_id: true        # pair with this to keep logs + spans together
```

```
Incoming batch (3 trace IDs across logs):
  logs with trace_id=aaa  →  partition key: "aaa..."  →  partition 0
  logs with trace_id=bbb  →  partition key: "bbb..."  →  partition 2
  logs with trace_id=ccc  →  partition key: "ccc..."  →  partition 0

Incoming batch (spans):
  spans of trace aaa  →  partition key: "aaa..."  →  partition 0  ← same partition as logs
```

---

#### Partition metrics by resource (keep a service's series together)

All metrics emitted by `host-1` go to the same partition, so a consumer building per-host time series only needs to read one partition.

```yaml
exporters:
  azure_event_hub:
    connection: "Endpoint=sb://<namespace>.servicebus.windows.net/;..."
    partition_metrics_by_resource_attributes: true
```

```
Incoming batch (2 resources):
  host-1 metrics  →  partition key: hash({host.name="host-1"})  →  partition 1
  host-2 metrics  →  partition key: hash({host.name="host-2"})  →  partition 3
```

---

#### Partition traces by ID (keep all spans of a trace together)

All spans belonging to trace `aaa` go to the same partition. A consumer reconstructing the full trace only needs to read one partition.

```yaml
exporters:
  azure_event_hub:
    connection: "Endpoint=sb://<namespace>.servicebus.windows.net/;..."
    partition_traces_by_id: true
```

```
Incoming batch (2 traces):
  spans of trace aaa  →  partition key: "aaa..."  →  partition 2
  spans of trace bbb  →  partition key: "bbb..."  →  partition 0
```

---

#### All three signals partitioned simultaneously

All flags are independent — enable all of them on a single exporter. Each signal is split by its own strategy and sent as separate messages.

```yaml
exporters:
  azure_event_hub:
    connection: "Endpoint=sb://<namespace>.servicebus.windows.net/;SharedAccessKeyName=<key>;SharedAccessKey=<secret>;EntityPath=otel-telemetry"
    partition_logs_by_resource_attributes: true   # group logs by service
    partition_metrics_by_resource_attributes: true # group metrics by host/service
    partition_traces_by_id: true                  # group spans by trace

service:
  pipelines:
    logs:
      receivers: [otlp]
      exporters: [azure_event_hub]
    metrics:
      receivers: [otlp]
      exporters: [azure_event_hub]
    traces:
      receivers: [otlp]
      exporters: [azure_event_hub]
```

---

## Rotating the client secret

1. In the Azure portal (or CLI), add a new credential to the App Registration **before** deleting the old one.
2. Update the `client_secret` value in your collector configuration and redeploy.
3. After confirming the collector is healthy, delete the old credential from the App Registration.

```bash
# Add a new secret
NEW_SECRET=$(az ad app credential reset \
  --id "$APP_ID" \
  --append \
  --years 1 \
  --query password -o tsv)
echo "New secret: $NEW_SECRET"

# List credentials to find the old keyId, then delete it
az ad app credential list --id "$APP_ID"
az ad app credential delete --id "$APP_ID" --key-id "<OLD_KEY_ID>"
```

---

## Migrating to user-assigned managed identity

If your collector runs on Azure compute (AKS, VM, Container Apps), you can eliminate the client secret entirely by switching to managed identity:

```yaml
extensions:
  azure_auth:
    managed_identity:
      client_id: "<MANAGED_IDENTITY_CLIENT_ID>"
    scopes:
      - "https://eventhubs.azure.net/.default"
```

Assign the managed identity the **Azure Event Hubs Data Sender** role on the namespace or hub (same `az role assignment create` command as step 3, replacing `$SP_OBJECT_ID` with the managed identity's principal ID).
