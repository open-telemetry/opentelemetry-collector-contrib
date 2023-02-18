# CloudWatch Metrics Receiver

| Status                   |               |
| ------------------------ | ------------- |
| Stability                | [development] |
| Supported pipeline types | metrics       |
| Distributions            | [contrib]     |

Receives Cloudwatch metrics from [AWS Cloudwatch](https://aws.amazon.com/cloudwatch/) via the [AWS SDK for Cloudwatch Logs](https://docs.aws.amazon.com/sdk-for-go/api/service/cloudwatchlogs/)

## Getting Started

This receiver uses the [AWS SDK](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html) as mode of authentication, which includes [Profile](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-profiles.html) and [IMDS](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html) authentication for EC2 instances.

## Configuration

### Top Level Parameters

| Parameter       | Notes      | type   | Description                                                                                                                                                                                                                                                                       |
| --------------- | ---------- | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `region`        | *required* | string | The AWS recognized region string                                                                                                                                                                                                                                                  |
| `profile`       | *optional* | string | The AWS profile used to authenticate, if none is specified the default is chosen from the list of profiles                                                                                                                                                                        |
| `IMDSEndpoint`       | *optional* | string | The IMDS endpoint to authenticate to AWS                                                                                                                                               |
| `poll_interval`          | `default=1m` | duration               | The duration waiting in between requests

| `nill_to_zero`          | `default=false` | duration               | Whether to send 0 if CloudWatch returns nodata. By default returns NaN

| `metrics`          | *optional* | `Metrics` | Configuration for metrics ingestion of this receiver                                                                                                                                                                                                                              |

### Metrics Parameters

| Parameter                | Notes        | type                   | Description                                                                                |
| ------------------------ | ------------ | ---------------------- | ------------------------------------------------------------------------------------------ |                                             |
 `named`                 | *optional*   | `See Group Parameters` | Configuration for Log Groups, by default no metrics will be collected |

### Group Parameters

`named` is the way to control and filter which metrics collected . They are mutually exclusive and are incompatible to be configured at the same time.

- `named`
  - This is a map of log group name to stream filtering options
    - `namespace`: (optional)
      - `metric_names`: A list of full log stream names to filter the discovered log groups to collect from.
      - `metric_name_prefixes`: A list of prefixes to filter the discovered log groups to collect from.


#### Named Example

```yaml
awscloudwatchmetrics:
  region: us-east-1
  poll_interval: 1m
  metrics:
    named:
      - namespace: "AWS/EC2"
        metric_name: "CPUUtilization"
        period: "5m"
        aws_aggregation: "Sum"
        dimensions:
          - Name: "InstanceId"
            Value: "i-1234567890abcdef0"
      - namespace: "AWS/S3"
        metric_name: "BucketSizeBytes"
        period: "5m"
        aws_aggregation: "p99"
        dimensions:
          - Name: "BucketName"
            Value: "OpenTelemetry"
          - Name: "StorageType"
            Value: "StandardStorage"
```

## Sample Configs

## AWS Costs

This receiver uses the [GetMetricData](https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_GetMetricData.html) API call, this call is *not* in the AWS free tier. Please refer to [Amazon's pricing](https://aws.amazon.com/cloudwatch/pricing/) for futher information about expected costs. For `us-east-1`, the current pricing is $0.01 per 1,000 metrics requested as of February 2023.


[alpha]:https://github.com/open-telemetry/opentelemetry-collector#alpha
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
[Issue]:https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/15667
