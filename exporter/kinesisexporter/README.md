# Kinesis Exporter

This exporter can be used to send traces to Kinesis.

The following configuration options are supported:

- `queue_size`: Dictates how many spans the queue should support. default: `100000` 
- `num_workers`: The number of workers to pick up span. default `8`.
- `flush_interval_seconds`: The interval time to flush span int memory. default：`5`.
- `max_bytes_per_batch`: The max list size. default: `100000`.
- `max_bytes_per_span`: The max allowed size per span. default：`900000`.
- `aws`.`kinesis_endpoint`: An optional endpoint URL (hostname only or fully qualified URI) that overrides the default generated endpoint for a client. Set this to `nil` or the value to `""` to use the default generated endpoint.
- `aws`.`region`: The region to send requests to.
- `aws`.`role`: Create the credentials from AssumeRoleProvider to assume the role.
- `aws`.`stream_name`: Set streamName for KinesisHooker.
- `kpl`.`aggregate_batch_count`: The maximum number of items to pack into an aggregated record.
- `kpl`.`aggregate_batch_size`: The maximum number of bytes to pack into an aggregated record.
- `kpl`.`batch_size`: The maximum number of bytes to send with a PutRecords request. Must not exceed 5MiB. default：`5MiB`.
- `kpl`.`batch_count`: The maximum number of items to pack in batch. default：`500`.
- `kpl`.`backlog_count`: The channel capacity before Put() will begin blocking. default：`batch_count`.
- `kpl`.`flush_interval_seconds`: FlushInterval is a regular interval for flushing the buffer. default：`5s`.
- `kpl`.`max_connections`: Maximum number of connections to open to the backend.
- `kpl`.`max_retries`: Maximum number of retries to connect to the backend.
- `kpl`.`max_backoff_seconds`: The Maximum seconds of backoff.

Example:

```yaml
exporters:
  kinesis:
    queue_size: 1
    num_workers: 2
    flush_interval_seconds: 3
    max_bytes_per_batch: 4
    max_bytes_per_span: 5

    aws:
        stream_name: test-stream
        region: mars-1
        role: arn:test-role
        kinesis_endpoint: kinesis.mars-1.aws.galactic

    kpl:
        aggregate_batch_count: 10
        aggregate_batch_size: 11
        batch_size: 12
        batch_count: 13
        backlog_count: 14
        flush_interval_seconds: 15
        max_connections: 16
        max_retries: 17
        max_backoff_seconds: 18
```

Beyond standard YAML configuration as outlined in the sections that follow,
exporters that leverage the net/http package (all do today) also respect the
following proxy environment variables:

* HTTP_PROXY
* HTTPS_PROXY
* NO_PROXY

If set at Collector start time then exporters, regardless of protocol,
will or will not proxy traffic as defined by these environment variables.