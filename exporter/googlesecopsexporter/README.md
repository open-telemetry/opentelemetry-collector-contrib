# Google SecOps Exporter

This exporter facilitates the sending of logs to [Google SecOps](https://cloud.google.com/security/products/security-operations) (previously Chronicle), a security analytics platform provided by Google. It is designed to integrate with OpenTelemetry collectors to export logs Google SecOps.

## Supported APIs

This exporter supports sending logs to Google SecOps using either of the following APIs
- [Chronicle API](https://docs.cloud.google.com/chronicle/docs/reference/ingestion-methods) (Preferred)
- [Backstory Ingestion API](https://docs.cloud.google.com/chronicle/docs/reference/ingestion-api)

## How It Works

1. The exporter uses the configured credentials to authenticate with the Google Cloud services.
2. Logs are marshalled into the format expected by Google SecOps.
3. Logs are imported into Google SecOps using the appropriate endpoint.

## Configuration

The exporter can be configured using the following fields:

| Field                      | Type              | Default                       | Required | Description                                                                                                                                              |
| -------------------------- | ----------------- | ----------------------------- | -------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `api`                      | string            | `chronicle`                   | `false`  | The API to use for sending logs. Valid values are `chronicle` and `backstory`.                                                                            |
| `hostname`                 | string            | `chronicle.googleapis.com`    | `false`  | The hostname used to construct the base URL for the API endpoints. Must not contain a protocol prefix.                                                   |
| `customer_id`              | string            |                               | `true`  | The customer ID used for sending logs.                                                                                                                   |
| `creds_file_path`          | string            |                               | `false`  | The file path to the Google credentials JSON file. Cannot be used with `creds`.                                                                          |
| `creds`                    | string            |                               | `false`  | The Google credentials JSON. Cannot be used with `creds_file_path`.                                                                                      |
| `default_log_type`         | string            |                               | `false`  | The type of log that will be sent if not overridden by `attributes["log_type"]`, `attributes["chronicle_log_type"]`, or `attributes["secops_log_type"]`. |
| `override_log_type`        | bool              | `true`                        | `false`  | Whether or not to override the `default_log_type` in the config with `attributes["log_type"]`.                                                           |
| `validate_log_types`       | bool              | `false`                       | `false`  | Whether or not to validate the log types using an API call.                                                                                              |
| `raw_log_field`            | string            |                               | `false`  | The field name for raw logs. Must be a valid OTTL log record expression.                                                                                 |
| `namespace`                | string            |                               | `false`  | User-configured environment namespace to identify the data domain the logs originated from.                                                              |
| `compression`              | string            | `none`                        | `false`  | The compression type to use when sending logs. Valid values are `none` and `gzip`.                                                                       |
| `ingestion_labels`         | map[string]string |                               | `false`  | Key-value pairs of labels to be applied to the logs when sent to Google SecOps.                                                                          |
| `collect_agent_metrics`    | bool              | `true`                        | `false`  | Enables collecting metrics about the agent's process and log ingestion metrics.                                                                          |
| `metrics_interval`         | duration          | `1m`                          | `false`  | The interval at which to collect and send agent metrics.                                                                                                 |
| `batch_request_size_limit` | int               | `4000000`                     | `false`  | The maximum batch request size, in bytes, that can be sent to Google SecOps. Must be a positive value.                                                   |
| `log_errored_payloads`     | bool              | `false`                       | `false`  | Whether or not to log errored payloads.                                                                                                                  |
| `location`                 | string            |                               | `true`*  | The location of the Google SecOps instance. Required for the Chronicle API.                                                                              |
| `project_number`           | string            |                               | `true`*  | The GCP project number of the Google SecOps instance. Required for the Chronicle API.                                                                    |
| `api_version`              | string            | `v1alpha`                     | `false`  | The API version to use. Valid values are `v1alpha` and `v1beta`. Only applies to the Chronicle API.                                                      |
| `override_hostname`        | bool              | `false`                       | `false`  | Whether or not to ignore the Location field when constructing the base URL. Only applies to the Chronicle API.                                           |

### Log Type

If the `attributes["log_type"]` field is present in the log, and maps to a known Chronicle `log_type` the exporter will use the value of that field as the log type. If the `attributes["log_type"]` field is not present, the exporter will use the value of the `log_type` configuration field as the log type.

currently supported log types are:

- windows_event.security
- windows_event.custom
- windows_event.application
- windows_event.system
- sql_server

If the `attributes["secops_log_type"]` field is present in the log, its value will be used as the Log Type in the payload instead of the automatic detection or the `log_type` in the config.

### Namespace and Ingestion Labels

If `attributes["secops_namespace"]` or `attributes["chronicle_namespace"]` is present in the log, its value will be used as the Namespace in the payload instead of the `namespace` in the config.

If there are nested fields in `attributes["secops_ingestion_label"]` or `attributes["chronicle_ingestion_label"]`, we will use the values in the payload instead of the `ingestion_labels` in the config.

## Credentials

This exporter requires a Google Cloud service account with access to the appropriate APIs.

The following IAM permissions are required for the Chronicle API:
- [chronicle.logs.import](https://docs.cloud.google.com/chronicle/docs/reference/rest/v1alpha/projects.locations.instances.logTypes.logs/import)
- [chronicle.forwarders.importStatsEvents](https://docs.cloud.google.com/chronicle/docs/reference/rest/v1alpha/projects.locations.instances.forwarders/importStatsEvents) When running the exporter with `collect_agent_metrics` enabled.
- [chronicle.logTypes.list](https://docs.cloud.google.com/chronicle/docs/reference/rest/v1alpha/projects.locations.instances.logTypes/list) When running the exporter with `validate_log_types` enabled.

For additional information on credentials, see the relevant documentation:
- For the [Chronicle API](https://docs.cloud.google.com/chronicle/docs/reference/authentication)
- For the [Backstory API](https://docs.cloud.google.com/chronicle/docs/reference/ingestion-api#getting_api_authentication_credentials).

Besides the default hostnames, there are also regional hostnames which can be used:
- For the [Chronicle API](https://docs.cloud.google.com/chronicle/docs/reference/rest?rep_location=us#regional-service-endpoint)
- For the [Backstory API](https://docs.cloud.google.com/chronicle/docs/reference/ingestion-api#regional_endpoints)

## Log Batch Creation Request Limits

`batch_request_size_limit` is used to ensure log batch creation requests don't exceed Google SecOps's backend limits. If a request exceeds the configured size limit, the request will be split into multiple requests that adhere to this limit, with each request containing a subset of the logs contained in the original request. Any single logs that result in the request exceeding the size limit will be dropped.

## Example Configurations

### Basic Chronicle API Configuration

```yaml
googlesecops:
  api: "chronicle"
  hostname: chronicle.googleapis.com
  location: "us"
  project_number: "123456789"
  customer_id: "customer-123"
  creds_file_path: "/path/to/google/creds.json"
  default_log_type: "ONEPASSWORD"
```

### Basic Backstory API Configuration

```yaml
googlesecops:
  api: "backstory"
  hostname: malachiteingestion-pa.googleapis.com
  customer_id: "customer-123"
  creds_file_path: "/path/to/google/creds.json"
  default_log_type: "WINEVTLOG"
```

### Backstory API with Regional Hostname

```yaml
googlesecops:
  api: "backstory"
  hostname: europe-malachiteingestion-pa.googleapis.com
  customer_id: "customer-123"
  creds_file_path: "/path/to/google/creds.json"
  default_log_type: "WINEVTLOG"
  namespace: "production"
```

### Chronicle API with Overridden Hostname

```yaml
googlesecops:
  api: "chronicle"
  hostname: chronicle.eu.rep.googleapis.com
  override_hostname: true
  location: "eu"
  project_number: "123456789"
  customer_id: "customer-123"
  creds_file_path: "/path/to/google/creds.json"
  default_log_type: "WINDOWS_DNS"
```

### Chronicle API with v1beta

```yaml
googlesecops:
  api: "chronicle"
  hostname: chronicle.googleapis.com
  api_version: "v1beta"
  location: "us"
  project_number: "123456789"
  customer_id: "customer-123"
  creds_file_path: "/path/to/google/creds.json"
  default_log_type: "WINEVTLOG"
```

### Configuration with Ingestion Labels

```yaml
googlesecops:
  api: "chronicle"
  hostname: chronicle.googleapis.com
  location: "us"
  project_number: "123456789"
  customer_id: "customer-123"
  creds_file_path: "/path/to/google/creds.json"
  default_log_type: "WINEVTLOG"
  namespace: "production"
  ingestion_labels:
    env: production
    zone: us-east-1
    team: security
```

### Configuration with Inline Credentials

```yaml
googlesecops:
  api: "chronicle"
  hostname: chronicle.googleapis.com
  location: "us"
  project_number: "123456789"
  customer_id: "customer-123"
  creds: '{"type":"service_account","project_id":"my-project", ...}'
  default_log_type: "ONEPASSWORD"
```

### Configuration with Gzip Compression

```yaml
googlesecops:
  api: "chronicle"
  hostname: chronicle.googleapis.com
  location: "us"
  project_number: "123456789"
  customer_id: "customer-123"
  creds_file_path: "/path/to/google/creds.json"
  default_log_type: "WINEVTLOG"
  compression: "gzip"
```

### Configuration with Raw Log Field

```yaml
googlesecops:
  api: "chronicle"
  hostname: chronicle.googleapis.com
  location: "us"
  project_number: "123456789"
  customer_id: "customer-123"
  creds_file_path: "/path/to/google/creds.json"
  default_log_type: "WINEVTLOG"
  raw_log_field: body["raw"]
```

### Configuration with Log Type Override Disabled

```yaml
googlesecops:
  api: "chronicle"
  hostname: chronicle.googleapis.com
  location: "us"
  project_number: "123456789"
  customer_id: "customer-123"
  creds_file_path: "/path/to/google/creds.json"
  default_log_type: "WINEVTLOG"
  override_log_type: false
```

### Configuration with Custom Batch Size and Metrics

```yaml
googlesecops:
  api: "chronicle"
  hostname: chronicle.googleapis.com
  location: "us"
  project_number: "123456789"
  customer_id: "customer-123"
  creds_file_path: "/path/to/google/creds.json"
  default_log_type: "WINEVTLOG"
  batch_request_size_limit: 2000000
  collect_agent_metrics: true
  metrics_interval: 5m
```

### Full Configuration with All Options

```yaml
googlesecops:
  api: "chronicle"
  hostname: chronicle.googleapis.com
  location: "us"
  project_number: "123456789"
  customer_id: "customer-123"
  creds_file_path: "/path/to/google/creds.json"
  api_version: "v1alpha"
  override_hostname: false
  default_log_type: "WINEVTLOG"
  override_log_type: true
  validate_log_types: true
  raw_log_field: body["raw"]
  namespace: "production"
  compression: "gzip"
  ingestion_labels:
    env: production
    zone: us-east-1
  collect_agent_metrics: true
  metrics_interval: 1m
  batch_request_size_limit: 4000000
  log_errored_payloads: false
  retry_on_failure:
    enabled: true
    max_elapsed_time: 300s
  sending_queue:
    enabled: true
    queue_size: 1000
  timeout: 30s
```
