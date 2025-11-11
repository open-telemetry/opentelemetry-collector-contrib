# hotreloadprocessor

The `hotreloadprocessor` is an OpenTelemetry Collector processor that enables dynamic, hot-reloadable configuration of selected processors (logs, metrics, traces) from encrypted configuration files stored in AWS S3. This allows you to update your observability pipelines without restarting the collector, making it ideal for dynamic environments.

## Features

* **Hot-reloads** processors configuration from S3 at runtime (default: every 60 seconds).
* Supports encrypted configuration files using a user-supplied encryption key.
* Works with logs, metrics, and traces processors.
* Supports any processor registered with the OpenTelemetry Collector via dynamic factory discovery.
* Emits internal telemetry for monitoring processor performance.

## Configuration

The processor is configured via three main fields:

```yaml
processors:
  hotreload:
    configuration_prefix: "s3://<bucket>/<prefix>"
    encryption_key: "<your-encryption-key>"
    region: "<aws-region>"
```

* **configuration\_prefix**: S3 URI prefix where configuration files are stored (e.g., `s3://my-bucket/my-prefix`).
* **encryption\_key**: Key used to decrypt the configuration files.
* **region**: AWS region of the S3 bucket.

### Example Pipeline

Suppose you have a configuration file in S3 (see [testdata/config.yaml](testdata/config.yaml) for an example):

```yaml
processors:
  transform/t1:
    log_statements:
      - context: log
        conditions:
          - IsString(body)
      - context: log
        statements:
          - set(attributes["pipeline"]["id"], String("12345"))
  filter/f1:
    error_mode: ignore
    logs:
      log_record:
        - 'attributes["pipeline"]["id"] == "12345"'
    sampling:
      percentage: 100
  batch/b1:
    send_batch_size: 1
    timeout: 0s
service:
  pipelines:
    logs/my-pipeline:
      processors:
        - transform/t1
        - filter/f1
        - batch/b1
```

## How It Works

1. On startup, the processor fetches the latest configuration from the specified S3 location, decrypts it, and applies it to the pipeline.
2. Every 60 seconds (configurable in code), it checks S3 for new configuration files and hot-reloads the pipeline if a new config is found.
3. Only one pipeline per config is supported.
4. If a config fails to load or apply, the processor logs the error and tries the next available config.

## Telemetry

The processor emits the following metric:

| Name                                   | Description                                      | Unit | Type      |
|-----------------------------------------|--------------------------------------------------|------|-----------|
| `otelcol_processor_hotreload_process_duration` | Duration of the processLogs function by the hotreloadprocessor | ms   | Histogram |

## Development & Testing

* The processor is in **development** status for logs, metrics, and traces.
* See `s3_test.go` for test cases and usage patterns.
* To run tests:
  ```sh
  go test ./...
  ```

## Dependencies

See [go.mod](go.mod) for a full list. Key dependencies include:

* AWS SDK v2
* OpenTelemetry Collector
* Standard OpenTelemetry processors (discovered dynamically at runtime)

## Limitations

* Only one pipeline per configuration file is supported.
* Configuration files must be encrypted and stored in S3.
* Only the listed processors are supported.
