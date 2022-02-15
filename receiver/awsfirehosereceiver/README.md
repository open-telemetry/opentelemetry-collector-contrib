# AWS Kinesis Data Firehose Receiver

Receiver for ingesting AWS Kinesis Data Firehose delivery stream messages and parsing the records received based on the configured encoding.

Supported pipeline types: metrics

## Configuration

Example:

```yaml
receivers:
  awsfirehose:
    endpoint: 0.0.0.0:443
    record_type: cwmetrics
    access_key: "some_access_key"
```

#### record_type:

The type of record being received from the delivery stream. Each unmarshaler handles a specific type, so the field allows the receiver to use the correct one.

default: `cwmetrics`

###### cwmetrics
The record type for the CloudWatch metric streams. Expects the format for the records to be JSON. See [documentation](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Metric-Streams.html) for details.

#### access_key:

The access key to be checked on each request received. This can be set when creating or updating the delivery stream. See [documentation](https://docs.aws.amazon.com/firehose/latest/dev/create-destination.html#create-destination-http) for details.