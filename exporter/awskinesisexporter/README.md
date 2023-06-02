# Kinesis Exporter

| Status                   |                       |
| ------------------------ |-----------------------|
| Stability                | [beta]                |
| Supported pipeline types | traces, logs, metrics |
| Distributions            | [contrib]             |

The kinesis exporter currently exports dynamic encodings to the configured kinesis stream.
The exporter relies heavily on the kinesis.PutRecords api to reduce network I/O and and reduces records into smallest atomic representation
to avoid hitting the hard limits placed on Records (No greater than 1Mb).
This producer will block until the operation is done to allow for retryable and queued data to help during high loads.

The following settings are required:
- `aws`
    - `stream_name` (no default): The name of the Kinesis stream to export to.

The following settings can be optionally configured:
- `aws`
    - `kinesis_endpoint` (no default)
    - `region` (default = us-west-2): the region that the kinesis stream is deployed in
    - `role` (no default): The role to be used in order to send data to the kinesis stream
- `encoding`
    - `name` (default = otlp): defines the export type to be used to send to kinesis (available is `otlp_proto`, `otlp_json`, `zipkin_proto`, `zipkin_json`, `jaeger_proto`)
      - **Note** : `otlp_json` is considered experimental and _should not_ be used for production environments. 
    - `compression` (default = none): allows to set the compression type (defaults BestSpeed for all) before forwarding to kinesis (available is `flate`, `gzip`, `zlib` or `none`)
- `max_records_per_batch` (default = 500, PutRecords limit): The number of records that can be batched together then sent to kinesis.
- `max_record_size` (default = 1Mb, PutRecord(s) limit on record size): The max allowed size that can be exported to kinesis
- `timeout` (default = 5s): Is the timeout for every attempt to send data to the backend.
- `retry_on_failure`
  - `enabled` (default = true)
  - `initial_interval` (default = 5s): Time to wait after the first failure before retrying; ignored if `enabled` is `false`
  - `max_interval` (default = 30s): Is the upper bound on backoff; ignored if `enabled` is `false`
  - `max_elapsed_time` (default = 120s): Is the maximum amount of time spent trying to send a batch; ignored if `enabled` is `false`
- `sending_queue`
  - `enabled` (default = true)
  - `num_consumers` (default = 10): Number of consumers that dequeue batches; ignored if `enabled` is `false`
  - `queue_size` (default = 1000): Maximum number of batches kept in memory before dropping data; ignored if `enabled` is `false`;
  User should calculate this as `num_seconds * requests_per_second` where:
    - `num_seconds` is the number of seconds to buffer in case of a backend outage
    - `requests_per_second` is the average number of requests per seconds.

Example Configuration:

```yaml
exporters:
  awskinesis:
    aws:
      stream_name: raw-trace-stream
      region: us-east-1
      role: arn:test-role
```

[beta]:https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
