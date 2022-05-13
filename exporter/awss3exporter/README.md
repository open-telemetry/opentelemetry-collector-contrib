# AWS S3 Exporter for OpenTelemetry Collector
This exporter converts OpenTelemetry metrics, logs and traces to supported format and upload to S3

## Schema supported
This exporter targets to support parquet/json format

## Exporter Configuration

The following exporter configuration parameters are supported. 

| Name                   | Description                                                                        | Default |
| :--------------------- | :--------------------------------------------------------------------------------- | ------- |
| `region`               | AWS region.                                                                        |         |
| `s3_bucket`            | S3 bucket                                                                          |         |
| `s3_prefix`            | prefix for the S3 key.                                                             |         |
| `s3_partition`         | time granularity of S3 key: hour or minute                                         |"minute" |
| `batch_count`          | max backoff seconds before next retry                                              |  1000   |

## AWS Credential Configuration

This exporter follows default credential resolution for the
[aws-sdk-go](https://docs.aws.amazon.com/sdk-for-go/api/index.html).

Follow the [guidelines](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html) for the
credential configuration.
