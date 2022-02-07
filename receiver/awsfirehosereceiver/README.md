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