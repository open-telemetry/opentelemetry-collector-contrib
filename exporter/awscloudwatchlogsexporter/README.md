# AWS CloudWatch Logs Exporter

AWS CloudWatch Logs Exporter sends logs data to AWS [CloudWatch Logs](https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/WhatIsCloudWatchLogs.html).
AWS credentials are retrieved from the [default credential chain](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials).
Region must be configured in the configuration if not set in the default credential chain.

NOTE: OpenTelemetry Logging support is experimental, hence this exporter is subject to change.

## Configuration

The following settings are required:

- `log_group_name`: The group name of the CloudWatch logs.
- `log_stream_name`: The stream name of the CloudWatch logs.

The following settings can be optionally configured:

- `region`: The AWS region where the log stream is in.
- `endpoint`: The CloudWatch Logs service endpoint which the requests are forwarded to. [See the CloudWatch Logs endpoints](https://docs.aws.amazon.com/general/latest/gr/cwl_region.html) for a list.

### Examples

Simplest configuration:

```yaml
exporters:
  awscloudwatchlogs:
    log_group_name: "testing-logs"
    log_stream_name: "testing-integrations-stream"
```

All configuration options:

```yaml
exporters:
  awscloudwatchlogs:
    log_group_name: "testing-logs"
    log_stream_name: "testing-integrations-stream"
    region: "us-east-1"
    endpoint: "logs.us-east-1.amazonaws.com"
    sending_queue:
      queue_size: 50
    retry_on_failure:
      enabled: true
      initial_interval: 10ms
```
