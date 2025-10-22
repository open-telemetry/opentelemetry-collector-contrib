# Hydrolix Exporter

| Status                   |           |
| ------------------------ | --------- |
| Stability                | [beta]    |
| Supported pipeline types | traces, metrics |
| Distributions            | []        |

The Hydrolix exporter sends traces and metrics to Hydrolix via HTTP.

## Getting Started

### Prerequisites

- A Hydrolix cluster with an ingest endpoint
- Hydrolix table and transform configured for telemetry data
- Authentication credentials (username and password)

### Usage

The exporter is used in an OpenTelemetry Collector pipeline to send observability data to Hydrolix. Add it to your collector configuration as shown below.

## Configuration

The following settings are required:

- `endpoint` (no default): The Hydrolix endpoint URL
- `hdx_table` (no default): The Hydrolix table name
- `hdx_transform` (no default): The Hydrolix transform name
- `hdx_username` (no default): Hydrolix authentication username
- `hdx_password` (no default): Hydrolix authentication password

The following settings can be optionally configured:

- `timeout` (default = 30s): HTTP request timeout
- `retry_on_failure`
  - `enabled` (default = true)
  - `initial_interval` (default = 5s)
  - `max_interval` (default = 30s)
  - `max_elapsed_time` (default = 5m)
- `sending_queue`
  - `enabled` (default = true)
  - `num_consumers` (default = 10)
  - `queue_size` (default = 1000)

## Example Configuration

```yaml
exporters:
  hydrolix:
    endpoint: https://your-hydrolix-endpoint.com/ingest
    hdx_table: your_table
    hdx_transform: your_transform
    hdx_username: your_username
    hdx_password: your_password
    timeout: 30s
```

[beta]: https://github.com/open-telemetry/opentelemetry-collector#beta