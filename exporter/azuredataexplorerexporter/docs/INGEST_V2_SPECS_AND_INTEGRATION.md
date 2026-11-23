# Ingest V2 integration

This guide describes how the Azure Data Explorer exporter ingests logs, metrics,
and traces through the Go Ingest V2 SDK.

## 1. Configuration

`enable_ingest_v2` is a per-exporter option, disabled by default. It allows V2
selection but does not force it. `ingestion_type: managed` always uses legacy
managed ingestion, even when the flag is enabled.

```yaml
exporters:
  azure_data_explorer:
    cluster_uri: "https://mycluster.kusto.windows.net"
    use_azure_auth: true
    db_name: "oteldb"
    ingestion_type: "queued"
    enable_ingest_v2: true
    metrics_table_name: "OTELMetrics"
    logs_table_name: "OTELLogs"
    traces_table_name: "OTELTraces"
    metrics_table_json_mapping: "otelmetrics_mapping"
    logs_table_json_mapping: "otellogs_mapping"
    traces_table_json_mapping: "oteltraces_mapping"
```

This example uses default Azure authentication. The existing service-principal,
system-assigned managed-identity, and user-assigned managed-identity settings are
also supported. Configure exactly one authentication method. The database,
tables, and referenced JSON mappings must already exist.

## 2. Endpoints and routing

The exporter derives the DM ingestion URL from `cluster_uri`, adding `ingest-`
to the hostname unless it is already present. Local/test addresses remain
unchanged.

```text
https://mycluster.kusto.windows.net
    -> https://ingest-mycluster.kusto.windows.net
```

The exporter reads configuration through the standalone Go SDK client:

```text
ConfigurationClient.GetConfigurationDetails(ctx)
    -> GET {dmURL}/v1/rest/ingestion/configuration
```

```text
enable_ingest_v2 = false                 -> legacy
ingestion_type = managed                -> legacy; no V2 configuration request
initial configuration unavailable       -> legacy
neither Rest ingestion nor Lake upload   -> legacy
queued + flag enabled + Rest or Lake    -> V2 queued ingestion
```

The preference fields are
`ingestionSettings.preferredIngestionMethod` and
`containerSettings.preferredUploadMethod`. Routing comparisons ignore case and
surrounding whitespace. `Rest` means queued REST ingestion, not streaming
ingestion.

Configuration and the V2 queued client are initialized lazily when data arrives.
Failure to create the queued client selects legacy before any V2 submission.

## 3. Configuration caching

Each signal's exporter producer maintains its own routing cache:

- Successful configuration uses the service's positive `refreshInterval`, capped
  at one hour; missing or unusable intervals use one hour.
- An initial load is attempted up to three times with two- and four-second
  backoffs. Timeouts are not retried; cancellation stops the request or backoff.
- An initial failure selects legacy and is cached for ten seconds.
- A refresh failure preserves the last successful configuration and retries the
  refresh after one minute.
- Concurrent routing reads are serialized.

The Go SDK's standalone configuration getter makes a fresh request per call.
The exporter caches the response for routing. The queued SDK client maintains
its own configuration cache for upload targets and ingestion limits.

## 4. Data serialization, staging, and submission

The exporter transforms all three signals into newline-delimited JSON.

```text
OTel records
    -> JSON transformation
    -> raw JSON StreamSource
    -> Go V2 SDK compression and staging upload
    -> queued REST ingestion submission
```

The stream is not marked as already gzipped; the SDK applies gzip compression.
Database, table, and the corresponding signal's JSON mapping reference are passed
to the SDK.

The SDK chooses upload targets from the DM response:

| Available targets | Staging destination |
|---|---|
| Only `containerSettings.containers` | Blob storage |
| Only `containerSettings.lakeFolders` | OneLake |
| Both, with `preferredUploadMethod` equal to `Lake` (case-insensitive) | OneLake |
| Both, with any other upload preference | Blob storage |
| Neither | Upload fails |

The URLs come from `containers[].path` or `lakeFolders[].path`. The exporter
does not construct a OneLake workspace or folder URL.

The exporter passes a stream, and the ingestion SDK performs staging upload and
submission internally. No connector-side staging upload is performed.

Staging upload is not Kusto streaming ingestion. V2 queued ingestion returns
after submission; the exporter does not poll for service-side completion.

## 5. Authentication and Private Link

The V2 path constructs Azure Identity credentials from the exporter
authentication settings.

| Authentication setting | V2 credential |
|---|---|
| `use_azure_auth: true` | Default Azure credential |
| `application_id`, `application_key`, `tenant_id` | Client-secret credential |
| `managed_identity_id: system` | System-assigned managed identity |
| `managed_identity_id: <client UUID>` | User-assigned managed identity |

For Kusto API calls, the exporter reads
`GET {dmURL}/v1/rest/auth/metadata`, validates the DM endpoint against the
metadata's login endpoint, and requests
`<KustoServiceResourceId>/.default` using the advertised resource ID.

Error HTTP statuses, empty metadata, or missing AzureAD settings use public
cloud defaults with a warning. Missing login or audience fields also use their
public defaults. Network, malformed-response, and body-read failures are returned
to configuration loading.
Successful credential and audience initialization is reused.

OneLake uploads use a separate Storage-audience token
(`https://storage.azure.com/.default`). The SDK requests that token from the same
configured identity, preserving the storage request's scopes and claims. Blob
uploads use the SAS URLs returned by the service.

The Collector needs network and DNS connectivity to the DM endpoint and the
service-returned upload endpoints. Enabling V2 does not configure or detect
Private Link.

## 6. Errors, fallback, and resource ownership

Configuration selection and queued-client creation can select legacy before V2
ingestion is attempted. Once the exporter calls V2 `Ingest`, upload and
submission errors are returned to the existing Collector retry machinery.
There is no automatic retry through the legacy ingestion client.

Cancellation is passed to configuration, authentication, and ingestion calls.
The SDK owns and closes the ingestion stream. Exporter shutdown closes the
legacy ingestor and any created V2 queued client, including its owned
configuration refresh.

## 7. Implementation map

| File | Responsibility |
|---|---|
| `config.go` | Default-off V2 option |
| `adx_exporter.go` | Existing signal transformation, stream dispatch, shutdown |
| `ingest_v2.go` | DM derivation, routing cache, queued-client creation, SDK ingestion |
| `ingest_v2_auth.go` | Metadata, Azure credentials, Kusto and storage token scopes |
