# Authentication

The Azure Event Hub exporter supports two authentication modes:

- **Connection string** — a SAS-based connection string from the Azure portal
- **Auth extension** — any `azcore.TokenCredential` supplied by an auth extension (service principal, managed identity, workload identity)

---

## Connection string

Paste the connection string directly from the Azure portal. The hub name can be embedded in the string via `EntityPath`, or supplied separately via `event_hub.name` (which takes precedence).

```yaml
exporters:
  azure_event_hub:
    connection: "Endpoint=sb://<namespace>.servicebus.windows.net/;SharedAccessKeyName=<key-name>;SharedAccessKey=<key>;EntityPath=<hub-name>"
```

Obtain the connection string from the Azure portal:

**Event Hubs namespace → Shared access policies → \<policy\> → Connection string–primary key**

or via CLI:

```bash
az eventhubs eventhub authorization-rule keys list \
  --resource-group <rg> \
  --namespace-name <namespace> \
  --eventhub-name <hub> \
  --name RootManageSharedAccessKey \
  --query primaryConnectionString -o tsv
```

---

## Auth extension (`azureauthextension`)

When using an auth extension, omit `connection` and instead supply `event_hub.namespace` and `event_hub.name`. The extension must implement `azcore.TokenCredential`; the [`azureauthextension`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/azureauthextension) satisfies this requirement.

The required OAuth scope for the Event Hubs data plane is `https://eventhubs.azure.net/.default`.

### Service principal (client secret)

Use this when your collector runs outside Azure (CI, on-premises, local dev) and you have an Azure AD App Registration.

**Prerequisites:**
1. An App Registration in Azure AD with a client secret
2. The service principal assigned the **Azure Event Hubs Data Sender** role on the namespace or hub

```yaml
extensions:
  azureauth:
    service_principal:
      tenant_id: ${env:AZURE_TENANT_ID}
      client_id: ${env:AZURE_CLIENT_ID}
      client_secret: ${env:AZURE_CLIENT_SECRET}
    scopes:
      - "https://eventhubs.azure.net/.default"

exporters:
  azure_event_hub:
    auth: azureauth
    event_hub:
      namespace: <namespace>.servicebus.windows.net
      name: <hub-name>

service:
  extensions: [azureauth]
  pipelines:
    logs:
      receivers: [otlp]
      exporters: [azure_event_hub]
```

Assign the RBAC role:

```bash
NAMESPACE_ID=$(az eventhubs namespace show \
  --resource-group <rg> --name <namespace> --query id -o tsv)

az role assignment create \
  --assignee <app-object-id> \
  --role "Azure Event Hubs Data Sender" \
  --scope "$NAMESPACE_ID"
```

See [README.md](./README.md) for the full step-by-step setup including App Registration creation.

---

### System-assigned managed identity

Use this when your collector runs on Azure compute (VM, AKS node pool, Container Apps) with a system-assigned identity enabled.

No credentials in the config — Azure injects the identity automatically.

```yaml
extensions:
  azureauth:
    managed_identity: {}
    scopes:
      - "https://eventhubs.azure.net/.default"

exporters:
  azure_event_hub:
    auth: azureauth
    event_hub:
      namespace: <namespace>.servicebus.windows.net
      name: <hub-name>
```

Assign the role to the compute resource's identity:

```bash
# For an AKS node pool
IDENTITY_ID=$(az aks show -g <rg> -n <cluster> \
  --query identityProfile.kubeletidentity.objectId -o tsv)

az role assignment create \
  --assignee "$IDENTITY_ID" \
  --role "Azure Event Hubs Data Sender" \
  --scope "$NAMESPACE_ID"
```

---

### User-assigned managed identity

Use this when you want explicit control over which identity is used (e.g., multiple identities on one VM, or shared identity across resources).

```yaml
extensions:
  azureauth:
    managed_identity:
      client_id: ${env:AZURE_CLIENT_ID}   # client ID of the user-assigned identity
    scopes:
      - "https://eventhubs.azure.net/.default"

exporters:
  azure_event_hub:
    auth: azureauth
    event_hub:
      namespace: <namespace>.servicebus.windows.net
      name: <hub-name>
```

Create and assign a user-assigned identity:

```bash
# Create the identity
az identity create --resource-group <rg> --name otel-collector-id

CLIENT_ID=$(az identity show -g <rg> -n otel-collector-id --query clientId -o tsv)
PRINCIPAL_ID=$(az identity show -g <rg> -n otel-collector-id --query principalId -o tsv)

# Assign Send permission
az role assignment create \
  --assignee "$PRINCIPAL_ID" \
  --role "Azure Event Hubs Data Sender" \
  --scope "$NAMESPACE_ID"
```

Then attach the identity to your compute resource (e.g., VM, VMSS, AKS pod via pod identity or workload identity).

---

### Workload identity (federated, AKS)

Use this for AKS workloads with Azure AD Workload Identity. It replaces pod-managed identity and avoids node-level credential exposure.

**Prerequisites:** AKS cluster with workload identity enabled, a federated credential on the App Registration or managed identity.

```yaml
extensions:
  azureauth:
    workload_identity:
      tenant_id: ${env:AZURE_TENANT_ID}
      client_id: ${env:AZURE_CLIENT_ID}
      federated_token_file: /var/run/secrets/azure/tokens/azure-identity-token
    scopes:
      - "https://eventhubs.azure.net/.default"

exporters:
  azure_event_hub:
    auth: azureauth
    event_hub:
      namespace: <namespace>.servicebus.windows.net
      name: <hub-name>
```

The `federated_token_file` path is injected by the AKS workload identity webhook when the pod has the `azure.workload.identity/use: "true"` label.

---

## Choosing the right method

| Environment | Recommended method |
|---|---|
| Local development / CI | Service principal (client secret) |
| Azure VM or VMSS | System-assigned managed identity |
| Shared or multi-tenant compute | User-assigned managed identity |
| AKS with workload identity | Workload identity (federated) |
| Any environment, simple setup | Connection string |

Connection strings are the easiest to get started with but embed credentials in the config. For production deployments on Azure compute, prefer managed identity or workload identity — they rotate automatically and require no secret management.
