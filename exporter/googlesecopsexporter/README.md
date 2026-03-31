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

| Field                      | Type              | Default                       | Required | Description                                                                                                                          |
| -------------------------- | ----------------- | ----------------------------- | -------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| `api`                      | string            | `chronicle`                   | `false`  | The API to use for sending logs. Valid values are `chronicle` and `backstory`.                                                       |
| `hostname`                 | string            | `chronicle.googleapis.com`    | `false`  | The hostname used to construct the base URL for the API endpoints. Must not contain a protocol prefix.                               |
| `customer_id`              | string            |                               | `true`  | The customer ID used for sending logs.                                                                                                |
| `creds_file_path`          | string            |                               | `false`  | The file path to the Google credentials JSON file. Cannot be used with `creds`.                                                      |
| `creds`                    | string            |                               | `false`  | The Google credentials JSON. Cannot be used with `creds_file_path`.                                                                  |
| `default_log_type`         | string            |                               | `false`  | The type of log that will be sent if not overridden by `attributes["chronicle_log_type"]` or `attributes["google_secops.log.type"]`. |
| `validate_log_types`       | bool              | `false`                       | `false`  | Whether or not to validate the log types using an API call.                                                                          |
| `raw_log_field`            | string            |                               | `false`  | The field name for raw logs. Must be a valid OTTL log record expression.                                                             |
| `namespace`                | string            |                               | `false`  | User-configured environment namespace to identify the data domain the logs originated from.                                          |
| `compression`              | string            | `none`                        | `false`  | The compression type to use when sending logs. Valid values are `none` and `gzip`.                                                   |
| `ingestion_labels`         | map[string]string |                               | `false`  | Key-value pairs of labels to be applied to the logs when sent to Google SecOps.                                                      |
| `collect_agent_metrics`    | bool              | `true`                        | `false`  | Enables collecting metrics about the agent's process and log ingestion metrics.                                                      |
| `metrics_interval`         | duration          | `1m`                          | `false`  | The interval at which to collect and send agent metrics.                                                                             |
| `batch_request_size_limit` | int               | `4000000`                     | `false`  | The maximum batch request size, in bytes, that can be sent to Google SecOps. Must be a positive value.                               |
| `log_errored_payloads`     | bool              | `false`                       | `false`  | Whether or not to log errored payloads.                                                                                              |
| `location`                 | string            |                               | `true`*  | The location of the Google SecOps instance. Required for the Chronicle API.                                                          |
| `project_number`           | string            |                               | `true`*  | The GCP project number of the Google SecOps instance. Required for the Chronicle API.                                                |
| `api_version`              | string            | `v1alpha`                     | `false`  | The API version to use. Valid values are `v1alpha` and `v1beta`. Only applies to the Chronicle API.                                  |
| `override_hostname`        | bool              | `false`                       | `false`  | Whether or not to ignore the Location field when constructing the base URL. Only applies to the Chronicle API.                       |

## Example Configurations

### Basic Chronicle API Configuration

```yaml
google_secops:
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
google_secops:
  api: "backstory"
  hostname: malachiteingestion-pa.googleapis.com
  customer_id: "customer-123"
  creds_file_path: "/path/to/google/creds.json"
  default_log_type: "WINEVTLOG"
```

### Backstory API with Regional Hostname

```yaml
google_secops:
  api: "backstory"
  hostname: europe-malachiteingestion-pa.googleapis.com
  customer_id: "customer-123"
  creds_file_path: "/path/to/google/creds.json"
  default_log_type: "WINEVTLOG"
  namespace: "production"
```

### Chronicle API with Overridden Hostname

```yaml
google_secops:
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
google_secops:
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
google_secops:
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
google_secops:
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
google_secops:
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
google_secops:
  api: "chronicle"
  hostname: chronicle.googleapis.com
  location: "us"
  project_number: "123456789"
  customer_id: "customer-123"
  creds_file_path: "/path/to/google/creds.json"
  default_log_type: "WINEVTLOG"
  raw_log_field: body["raw"]
```

### Configuration with Custom Batch Size and Metrics

```yaml
google_secops:
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
google_secops:
  api: "chronicle"
  hostname: chronicle.googleapis.com
  location: "us"
  project_number: "123456789"
  customer_id: "customer-123"
  creds_file_path: "/path/to/google/creds.json"
  api_version: "v1alpha"
  override_hostname: false
  default_log_type: "WINEVTLOG"
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
