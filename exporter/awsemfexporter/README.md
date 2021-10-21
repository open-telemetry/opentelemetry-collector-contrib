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
| `log_group_name`  | Customized log group name which supports `{ClusterName}` and `{TaskId}` placeholders. One valid example is `/aws/metrics/{ClusterName}`. It will search for `ClusterName` (or `aws.ecs.cluster.name`) resource attribute in the metrics data and replace with the actual cluster name. If none of them are found in the resource attribute map, `{ClusterName}` will be replaced by `undefined`. Similar way, for the `{TaskId}`, it searches for `TaskId` (or `aws.ecs.task.id`) key in the resource attribute map. For `{NodeName}`, it searches for `NodeName` (or `k8s.node.name`)                                         |"/metrics/default"|
| `log_stream_name` | Customized log stream name which supports `{TaskId}`, `{ClusterName}`, `{NodeName}`, `{ContainerInstanceId}`, and `{TaskDefinitionFamily}` placeholders. One valid example is `{TaskId}`. It will search for `TaskId` (or `aws.ecs.task.id`) resource attribute in the metrics data and replace with the actual task id. If none of them are found in the resource attribute map, `{TaskId}` will be replaced by `undefined`. Similarly, for the `{TaskDefinitionFamily}`, it searches for `TaskDefinitionFamily` (or `aws.ecs.task.family`). For the `{ClusterName}`, it searches for `ClusterName` (or `aws.ecs.cluster.name`). For `{NodeName}`, it searches for `NodeName` (or `k8s.node.name`). For `{ContainerInstanceId}`, it searches for `ContainerInstanceId` (or `aws.ecs.container.instance.id`). (Note: ContainerInstanceId (or `aws.ecs.container.instance.id`) only works for AWS ECS EC2 launch type.                                            |"otel-stream"|
| `namespace`       | Customized CloudWatch metrics namespace                                | "default" |
| `endpoint`        | Optionally override the default CloudWatch service endpoint.           |         |
| `no_verify_ssl`   | Enable or disable TLS certificate verification.                        | false   |
| `proxy_address`   | Upload Structured Logs to AWS CloudWatch through a proxy.              |         |
| `region`          | Send Structured Logs to AWS CloudWatch in a specific region. If this field is not present in config, environment variable "AWS_REGION" can then be used to set region.| determined by metadata |
| `role_arn`        | IAM role to upload segments to a different account.                    |         |
| `max_retries`     | Maximum number of retries before abandoning an attempt to post data.   |    1    |
| `dimension_rollup_option`| DimensionRollupOption is the option for metrics dimension rollup. Three options are available. |"ZeroAndSingleDimensionRollup" (Enable both zero dimension rollup and single dimension rollup)| 
| `resource_to_telemetry_conversion` | "resource_to_telemetry_conversion" is the option for converting resource attributes to telemetry attributes. It has only one config onption- `enabled`. For metrics, if `enabled=true`, all the resource attributes will be converted to metric labels by default. See `Resource Attributes to Metric Labels` section below for examples. | `enabled=false` | 
| `output_destination` | "output_destination" is an option to specify the EMFExporter output. Currently, two options are available. "cloudwatch" or "stdout" | `cloudwatch` | 
| `parse_json_encoded_attr_values` | List of attribute keys whose corresponding values are JSON-encoded strings and will be converted to  JSON structures in emf logs. For example, the attribute string value "{\\"x\\":5,\\"y\\":6}" will be converted to a json object: ```{"x": 5, "y": 6}```| [ ] | 
| [`metric_declarations`](#metric_declaration) | List of rules for filtering exported metrics and their dimensions. |    [ ]   |
| [`metric_descriptors`](#metric_descriptor) | List of rules for inserting or updating metric descriptors.| [ ]|

### <metric_declaration>
A metric_declaration section characterizes a rule to be used to set dimensions for exported metrics, filtered by the incoming metrics' labels and metric names.

| Name              | Description                                                            | Default |
| :---------------- | :--------------------------------------------------------------------- | ------- |
| `dimensions`      | List of dimension sets to be exported.                                 |  [[ ]]   |
| `metric_name_selectors` | List of regex strings to filter metric names by.                 |         |
| [`label_matchers`](#label_matcher)  | (Optional) list of label matching rules to filter metrics by their labels. This rule is applied to any metric that matches any of the label matchers. |   [ ]    |

#### <label_matcher>
A label_matcher section defines a matching rule against the labels of the incoming metric. Only metrics that match the rules will be used by the surrounding `metric_declaration`.

| Name              | Description                                                            | Default |
| :---------------- | :--------------------------------------------------------------------- | ------- |
| `label_names`     | List of label names to filter by. Their corresponding values are concatenated using the separator and matched against the configured regular expression.                             |         |
| `separator`       | (Optional) separator placed between concatenated label values.         |   ";"   |
| `regex`           | Regex string to be matched against concatenated label values.          |         |

### <metric_descriptor>
A metric descriptor section allows the schema of a metric to be overwritten before sending out to the CloudWatch backend service. Currently, we only support unit override.

| Name              | Description                                                            | Default |
| :---------------- | :--------------------------------------------------------------------- | ------- |
| `metric_name`      | The name of the metric to be overwritten.                             |         |
| `unit` | The overwritten value of unit. The [MetricDatum](https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_MetricDatum.html) contains a ful list of supported unit values. |         |
| `overwrite` | `true` if the schema should be overwritten with the given specification, otherwise it will only be configured if empty. |   false   |


## AWS Credential Configuration

This exporter follows default credential resolution for the 
[aws-sdk-go](https://docs.aws.amazon.com/sdk-for-go/api/index.html).

Follow the [guidelines](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html) for the 
credential configuration.


## Configuration Examples


### Resource Attributes to Metric Labels
`resource_to_telemetry_conversion`  option can be enabled to convert all the resource attributes to metric labels. By default, this option is disabled. Users need to set `enabled=true` to opt-in. See the config example below.

```yaml
exporters:
    awsemf:
        region: 'us-west-2'
        resource_to_telemetry_conversion:
            enabled: true
```
