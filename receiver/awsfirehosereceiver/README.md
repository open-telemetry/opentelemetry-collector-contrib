# AWS Kinesis Data Firehose Receiver

Receiver for ingesting AWS Kinesis Data Firehose delivery stream messages and parsing the records received based on the configured encoding.

Supported pipeline types: metrics

## Configuration

Example:

```yaml
receivers:
  awsfirehose:
    endpoint: 0.0.0.0:443
    encoding: cwmetrics
```

#### encoding:

The encoding to use for the records received. Corresponds to an unmarshaler.

default: `cwmetrics`

###### cwmetrics
The encoding for the CloudWatch metric streams. Expects the format for the records to be JSON. See [documentation](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Metric-Streams.html) for details.

#### access_key:

The access key to be checked on each request received. This can be set when creating or updating the delivery stream.