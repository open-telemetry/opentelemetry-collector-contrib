# Amazon Elastic Container Service Observer

**Status: beta**

The `ecsobserver` uses the ECS/EC2 API to discover prometheus scrape target from all running tasks and filter them based
on service names, task definitions and container labels.

## Config

The configuration is based on
[existing cloudwatch agent ECS discovery](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/ContainerInsights-Prometheus-Setup-autodiscovery-ecs.html)
. A full collector config looks like the following:

```yaml
extensions:
  ecs_observer:
    refresh_interval: 15s
    cluster_name: 'Cluster-1'
    cluster_region: 'us-west-2'
    result_file: '/etc/ecs_sd_targets.yaml'
    sevice:
      - name_pattern: '^retail-.*$'
    docker_labels:
      - port_label: 'ECS_PROMETHEUS_EXPORTER_PORT'
    task_definitions:
      - job_name: 'task_def_1'
        metrics_path: '/metrics'
        metrics_ports:
          - 9113
          - 9090
        arn_pattern: '.*:task-definition/nginx:[0-9]+'

receivers:
  prometheus:
    config:
      scrape_configs:
        - job_name: "ecs-task"
          file_sd_configs:
            - files:
                - '/etc/ecs_sd_targets.yaml'

processors:
  batch:

exporters:
  awsemf:

service:
  pipelines:
    metrics:
      receivers: [ prometheus ]
      processors: [ batch ]
      exporters: [ awsemf ]
  extensions: [ ecs_observer ]
```

| Name             |           | Description                                                                                      |
|------------------|-----------|--------------------------------------------------------------------------------------------------|
| cluster_name     | Mandatory | target ECS cluster name for service discovery                                                    |
| cluster_region   | Mandatory | target ECS cluster's AWS region name                                                             |
| refresh_interval | Optional  | how often to look for changes in endpoints (default: 10s)                                        |
| result_file      | Optional  | path of YAML file for the scrape target results. When enabled, the observer always returns empty |
| services         | Optional  | list of service name patterns [detail](#ecs-service-name-based-filter-configuration)             |
| task_definitions | Optional  | list of task definition arn patterns [detail](#ecs-task-definition-based-filter-configuration)   |
| docker_labels    | Optional  | list of docker labels [detail](#docker-label-based-filter-configuration)                         |

### Output configuration

`result_file` specifies where to write the discovered targets. It MUST match the files defined in `file_sd_configs` for
prometheus receiver.

### Filters configuration

There are three type of filters, and they share the following common optional properties.

- `job_name`
- `metrics_path`
- `metrics_ports` an array of port number

Example

```yaml
ecs_observer:
  job_name: 'ecs-sd-job'
  services:
    - name_pattern: ^retail-.*$
      container_name_pattern: ^java-api-v[12]$
    - name_pattern: game
      metrics_path: /v3/343
      job_name: guilty-spark
  task_definitions:
    - arn_pattern: '*memcached.*'
    - arn_pattern: '^proxy-.*$`
      metrics_ports:
        - 9113
        - 9090
      metrics_path: /internal/metrics
  docker_labels:
    - port_label: ECS_PROMETHEUS_EXPORTER_PORT
    - port_label: ECS_PROMETHEUS_EXPORTER_PORT_V2
      metrics_path_label: ECS_PROMETHEUS_EXPORTER_METRICS_PATH
```

#### ECS Service Name based filter Configuration

| Name                   |           | Description                                                                                        |
|------------------------|-----------|----------------------------------------------------------------------------------------------------|
| name_pattern           | Mandatory | Regex pattern to match against ECS service name                                                    |
| metrics_ports          | Mandatory | container ports separated by semicolon. Only containers that expose these ports will be discovered |
| container_name_pattern | Optional  | ECS task container name regex pattern                                                              |

#### ECS Task Definition based filter Configuration

Specify label keys to look up value

| Name               |           | Description                                                                     |
|--------------------|-----------|---------------------------------------------------------------------------------|
| port_label         | Mandatory | container's docker label name that specifies the metrics port                   |
| metrics_path_label | Optional  | container's docker label name that specifies the metrics path. (Default: "")    |
| job_name_label     | Optional  | container's docker label name that specifies the scrape job name. (Default: "") |

#### Docker Label based filter Configuration

| Name                   |           | Description                                                                                        |
|------------------------|-----------|----------------------------------------------------------------------------------------------------|
| arn_pattern            | Mandatory | Regex pattern to match against ECS task definition ARN                                             |
| metrics_ports          | Mandatory | container ports separated by semicolon. Only containers that expose these ports will be discovered |
| container_name_pattern | Optional  | ECS task container name regex pattern                                                              |

### Authentication

Currently, only [ECS task role](https://docs.aws.amazon.com/AmazonECS/latest/userguide/task-iam-roles.html) is
supported. You need to deploy the collector as an ECS task/service with
the [following permissions](https://docs.amazonaws.cn/en_us/AmazonCloudWatch/latest/monitoring/ContainerInsights-Prometheus-install-ECS.html#ContainerInsights-Prometheus-Setup-ECS-IAM)
.

```text
ec2:DescribeInstances
ecs:ListTasks
ecs:ListServices
ecs:DescribeContainerInstances
ecs:DescribeServices
ecs:DescribeTasks
ecs:DescribeTaskDefinition
```

## Design

- [Discovery](#discovery-mechanism)
- [Notify receiver](#notify-prometheus-receiver-of-discovered-targets)

### Discovery mechanism

The extension polls ECS API periodically to get all running tasks and filter out scrape targets. There are 3 types of
filters for discovering targets, targets match the filter are kept. Targets from different filters are merged base
on `address/metrics_path` before updating/creating receiver.

#### ECS Service Name based filter

ECS [Service](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_services.html) is a deployment that
manages multiple tasks with same [definition](#ecs-task-definition-based-filter) (like Deployment and DaemonSet in k8s).

The `service`
configuration matches both service name and container name (if not empty).

NOTE: name of the service is **added** as label value with key `ServiceName`.

```yaml
# Example 1: Matches all containers that are started by retail-* service
name_pattern: ^retail-.*$
---
# Example 2: Matches all container with name java-api in cash-app service 
name_pattnern: ^cash-app$
container_name_pattern: ^java-api$
---
# Example 3: Override default metrics_path (i.e. /metrics)
name_pattern: ^log-replay-worker$
metrics_path: /v3/metrics
```

### ECS Task Definition based filter

ECS [task definition](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_definitions.html) contains one or
more containers (like Pod in k8s). Long running applications normally uses [service](#ecs-service-name-based-filter).
while short running (batch) jobs can
be [created from task definitions directly](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/scheduling_tasks.html)
.

The `task` definition matches both task definition name and container name (of not empty). Optional config
like `metrics_path`, `metrics_ports`, `job_name` can override default value.

```yaml
# Example 1: Matches all the tasks created from task definition that contains memcached in its arn
arn_pattern: "*memcached.*"
```

### Docker Label based filter

Docker label can be specified in task definition. Only `port_label` is used when checking if the container should be
included. Optional config like `metrics_path_label`, `job_name_label` can override default value.

```yaml
# Example 1: Matches all the container that has label ECS_PROMETHEUS_EXPORTER_PORT_NGINX
port_label: 'ECS_PROMETHEUS_EXPORTER_PORT_NGINX'
---
# Example 2: Override job name based on label MY_APP_JOB_NAME
port_label: 'ECS_PROMETHEUS_EXPORTER_PORT_MY_APP'
job_name_label: 'MY_APP_JOB_NAME'
```

### Notify Prometheus Receiver of discovered targets

There are three ways to notify a receiver

- Use [file based service discovery](#generate-target-file-for-file-based-discovery) in prometheus config and updates
  the file.
- Use [receiver creator framework](#receiver-creator-framework) to create a new receiver for new endpoints.
- Register as a prometheus discovery plugin.

#### Generate target file for file based discovery

- Status: implemented

This is current approach used by cloudwatch-agent and
also [recommended by prometheus](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#file_sd_config)
. It's easier to debug and the main drawback is it only works for prometheus. Another minor issue is fsnotify may not
work properly occasionally and delay the update.

#### Receiver creator framework

- Status: pending

This is a generic approach that creates a new receiver at runtime based on discovered endpoints. The main problem is
performance issue as described
in [this issue](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/1395).

#### Register as prometheus discovery plugin

- Status: pending

Because both the collector and prometheus is written in Go, we can call `discover.RegisterConfig` to make it a valid
config for prometheus (like other in tree plugins like kubernetes). The drawback is the configuration is now under
prometheus instead of extension and can cause confusion.

## Known issues

- There is no list watch API in ECS (unlike k8s), and we fetch ALL the tasks and filter it locally. If the poll interval
  is too short or there are multiple instances doing discovery, you may hit the (undocumented) API rate limit.
- A single collector may not be able to handle a large cluster, you can use `hashmod`
  in [relabel_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config) to do
  static sharding. However, it may trigger the rate limit on AWS API as each shard is fetching ALL the tasks during
  discovery regardless of mod factor.

## Implementation

The implementation has two parts, core ecs service discovery logic and adapter for notifying discovery results.

- `extension/observer/ecsobserver` adapter to implement the observer interface
- `internal/awsecs` polling AWS ECS and EC2 API and filter based on config
- `internal/awsconfig` the shared aws specific config (e.g. init sdk client), which eventually should be shared by every
  package that calls AWS API (e.g. emf, xray).

The pseudo code showing the overall flow.

```
NewECSSD() {
  session := awsconfig.NewSssion()
  ecsClient := awsecs.NewClient(session)
  filters := config.NewFileters()
  decorator := awsec2.NewClient(session)
  for {
    select {
    case <- timer:
      // Fetch ALL
      tasks := ecsClient.FaetchAll()
      // Filter
      filteredTasks := fileters.Apply(tasks)
      // Add EC2 info
      decorator.Apply(filteredTask)
      // Generate output
      if writeResultFile {
         writeFile(fileteredTasks, /etc/ecs_sd.yaml)
      } else {
          notifyObserver()
      }
    }
  }
}
```

### Output format

In cloudwatch agent's implementation, `__meta__` label is not used and all the ECS and EC2 information are attached as
normal label. TODO(pingleig): maybe we should change this behaviour to align with other service discovery
implementations.

```yaml
- targets:
    - 10.6.1.95:32785
  labels:
    __metrics_path__: /metrics
    ECS_PROMETHEUS_EXPORTER_PORT_SUBSET_B: "9406"
    ECS_PROMETHEUS_JOB_NAME: demo-jar-ec2-bridge-subset-b-dynamic
    ECS_PROMETHEUS_METRICS_PATH: /metrics
    InstanceType: t3.medium
    LaunchType: EC2
    SubnetId: subnet-0347624eeea6c5969
    TaskDefinitionFamily: demo-jar-ec2-bridge-dynamic-port-subset-b
    TaskGroup: family:demo-jar-ec2-bridge-dynamic-port-subset-b
    TaskRevision: "7"
    VpcId: vpc-033b021cd7ecbcedb
    container_name: demo-jar-ec2-bridge-dynamic-port-subset-b
    job: task_def_2
- targets:
    - 10.6.1.95:32783
  labels:
    __metrics_path__: /metrics
    ECS_PROMETHEUS_EXPORTER_PORT_SUBSET_B: "9406"
    ECS_PROMETHEUS_JOB_NAME: demo-jar-ec2-bridge-subset-b-dynamic
    ECS_PROMETHEUS_METRICS_PATH: /metrics
    InstanceType: t3.medium
    LaunchType: EC2
    SubnetId: subnet-0347624eeea6c5969
    TaskDefinitionFamily: demo-jar-ec2-bridge-dynamic-port-subset-b
    TaskGroup: family:demo-jar-ec2-bridge-dynamic-port-subset-b
    TaskRevision: "7"
    VpcId: vpc-033b021cd7ecbcedb
    container_name: demo-jar-ec2-bridge-dynamic-port-subset-b
    job: task_def_2
```
