# AWS CloudWatch EMF Exporter for OpenTelemetry Collector

This exporter converts OpenTelemetry metrics to 
[AWS CloudWatch Embedded Metric Format(EMF)](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch_Embedded_Metric_Format_Specification.html)
and then sends them directly to CloudWatch Logs using the 
[PutLogEvents](https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_PutLogEvents.html) API.

## Data Conversion
Convert OpenTelemetry ```Int64DataPoints```, ```DoubleDataPoints```, ```SummaryDataPoints``` metrics datapoints into CloudWatch ```EMF``` structured log formats and send it to CloudWatch. Logs and Metrics will be displayed in CloudWatch console.

## Exporter Configuration

The following exporter configuration parameters are supported.

| Name              | Description                                                            | Default |
| :---------------- | :--------------------------------------------------------------------- | ------- |
| `log_group_name`  | Customized log group name                                              |"/metrics/default"|
| `log_stream_name` | Customized log stream name                                             |"otel-stream"|
| `namespace`       | Customized CloudWatch metrics namespace                                | "default" |
| `endpoint`        | Optionally override the default CloudWatch service endpoint.           |         |
| `no_verify_ssl`   | Enable or disable TLS certificate verification.                        | false   |
| `proxy_address`   | Upload Structured Logs to AWS CloudWatch through a proxy.              |         |
| `region`          | Send Structured Logs to AWS CloudWatch in a specific region. If this field is not present in config, environment variable "AWS_REGION" can then be used to set region.| determined by metadata |
| `role_arn`        | IAM role to upload segments to a different account.                    |         |
| `max_retries`     | Maximum number of retries before abandoning an attempt to post data.   |    1    |
| `dimension_rollup_option`| DimensionRollupOption is the option for metrics dimension rollup. Three options are available. |"ZeroAndSingleDimensionRollup" (Enable both zero dimension rollup and single dimension rollup)|


## AWS Credential Configuration

This exporter follows default credential resolution for the 
[aws-sdk-go](https://docs.aws.amazon.com/sdk-for-go/api/index.html).

Follow the [guidelines](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html) for the 
credential configuration.