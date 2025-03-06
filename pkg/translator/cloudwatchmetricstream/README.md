# Cloudwatch metric stream translator

The translator for [cloudwatch metric stream](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Metric-Streams.html) 
extracts the OpenTelemetry metrics from a record containing cloudwatch metric streams.

The record can contain several metrics, each split by a new line.

Currently, there is support for records in JSON format.