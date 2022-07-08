# AWS CloudWatch Logs Exporter

| Status                   |           |
| ------------------------ |-----------|
| Stability                | [beta]    |
| Supported pipeline types | logs      |
| Distributions            | [contrib] |

AWS CloudWatch Logs Exporter sends logs data to AWS [CloudWatch Logs](https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/WhatIsCloudWatchLogs.html).
AWS credentials are retrieved from the [default credential chain](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials).
Region must be configured in the configuration if not set in the default credential chain.

NOTE: OpenTelemetry Logging support is experimental, hence this exporter is subject to change.

## Configuration

The following settings are required:

| Name                                         | Description                                                            | Default |
|:---------------------------------------------| :--------------------------------------------------------------------- | ------- |
| `log_group_name`                             | Customized log group name which supports `{ClusterName}` and `{TaskId}` placeholders. One valid example is `/aws/metrics/{ClusterName}`. It will search for `ClusterName` (or `aws.ecs.cluster.name`) resource attribute in the metrics data and replace with the actual cluster name. If none of them are found in the resource attribute map, `{ClusterName}` will be replaced by `undefined`. Similar way, for the `{TaskId}`, it searches for `TaskId` (or `aws.ecs.task.id`) key in the resource attribute map. For `{NodeName}`, it searches for `NodeName` (or `k8s.node.name`)                                         |
| `log_stream_name`                            | Customized log stream name which supports `{TaskId}`, `{ClusterName}`, `{NodeName}`, `{ContainerInstanceId}`, and `{TaskDefinitionFamily}` placeholders. One valid example is `{TaskId}`. It will search for `TaskId` (or `aws.ecs.task.id`) resource attribute in the metrics data and replace with the actual task id. If none of them are found in the resource attribute map, `{TaskId}` will be replaced by `undefined`. Similarly, for the `{TaskDefinitionFamily}`, it searches for `TaskDefinitionFamily` (or `aws.ecs.task.family`). For the `{ClusterName}`, it searches for `ClusterName` (or `aws.ecs.cluster.name`). For `{NodeName}`, it searches for `NodeName` (or `k8s.node.name`). For `{ContainerInstanceId}`, it searches for `ContainerInstanceId` (or `aws.ecs.container.instance.id`). (Note: ContainerInstanceId (or `aws.ecs.container.instance.id`) only works for AWS ECS EC2 launch type.                                            |

Optional:
- `region`The AWS region where the log stream is in.
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

[beta]:https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
