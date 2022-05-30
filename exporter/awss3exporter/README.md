# AWS S3 Exporter for OpenTelemetry Collector

| Status                   |                       |
| ------------------------ |-----------------------|
| Stability                | [in development]      |
| Supported pipeline types | traces, logs          |
| Distributions            | [contrib]             |

## Schema supported
This exporter targets to support proto/json and proto/binary format

## Exporter Configuration

The following exporter configuration parameters are supported. 

| Name                   | Description                                                                        | Default |
| :--------------------- | :--------------------------------------------------------------------------------- | ------- |
| `region`               | AWS region.                                                                        |         |
| `s3_bucket`            | S3 bucket                                                                          |         |
| `s3_prefix`            | prefix for the S3 key (root directory inside bucket).                              |         |
| `s3_partition`         | time granularity of S3 key: hour or minute                                         |"minute" |
| `file_prefix`          | file prefix defined by user                                                        |         |
| `marshaler_name`       | marshaler used to produce output data otlp_json or otlp_proto                      |         |

# Example Configuration

Following example configuration defines to store output in 'eu-central' region and bucket named 'databucket'.

```yaml
exporters:
  awss3:
    s3uploader:
        region: 'eu-central-1'
        s3_bucket: 'databucket'
        s3_prefix: 'metric'
        s3_partition: 'minute'
```

Logs and traces will be stored inside 'databucket' in the following path format.

```console
metric/year=XXXX/month=XX/day=XX/hour=XX/minute=XX
```

## AWS Credential Configuration

This exporter follows default credential resolution for the
[aws-sdk-go](https://docs.aws.amazon.com/sdk-for-go/api/index.html).

Follow the [guidelines](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html) for the
credential configuration.
