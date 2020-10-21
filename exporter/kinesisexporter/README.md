# Kinesis Exporter

Kinesis exporter exports OpenTelemetry data to Kinesis. This exporter uses a [KPL][kpl-url]-like batch producer and uses
the same aggregation format that KPLs use. Message payload encoding is configurable.

The following settings can be optionally configured:
- `aws` contains AWS specific configuration
  - `stream_name` (default = test-stream): The name of the Kinesis stream where events are sent/pushed
  - `kinesis_endpoint`: The Kinesis endpoint if role is not being assumed
  - `region` (default = us-west-2): The AWS region where the Kinesis stream is defined
  - `role`: The Kinesis role to assume
- `kpl` contains kinesis producer library related config to controls things like aggregation, batching, connections, retries, etc
  - `aggregate_batch_count` (default = 4294967295): Determines the maximum number of items to pack into an aggregated record. Must not exceed 4294967295
  - `aggregate_batch_size` (default = 51200): Determines the maximum number of bytes to pack into an aggregated record. User records larger than this will bypass aggregation
  - `batch_size` (default = 5242880): Determines the maximum number of bytes to send with a PutRecords request. Must not exceed 5MiB
  - `batch_count` (default = 1000): Determines the maximum number of items to pack in the batch. Must not exceed 1000
  - `backlog_count` (default = 2000): Determines the channel capacity before Put() will begin blocking. Default to `BatchCount`
  - `flush_interval_seconds` (default = 5): The regular interval for flushing the kinesis producer buffer
  - `max_connections` (default = 24): Number of requests to send concurrently
  - `max_retries` (default = 10): Number of retry attempts to make before dropping records
  - `max_backoff_seconds` (default = 60): Maximum time to backoff. Must be greater than 1s

Example configuration:

```yaml
exporters:
  kinesis:
      aws:
          stream_name: test-stream
          region: mars-1
          role: arn:test-role
          kinesis_endpoint: kinesis.mars-1.aws.galactic
      kpl:
          aggregate_batch_count: 4294967295
          aggregate_batch_size: 51200
          batch_size: 5242880
          batch_count: 1000
          backlog_count: 2000
          flush_interval_seconds: 5
          max_connections: 24
          max_retries: 10
          max_backoff_seconds: 60
```

[kpl-url]: https://github.com/awslabs/amazon-kinesis-producer