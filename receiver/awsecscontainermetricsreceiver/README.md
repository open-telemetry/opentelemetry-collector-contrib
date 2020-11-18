# AWS ECS Container Metrics Receiver

***Status:***
This receiver is under development and not recommended for production usage.

## Overview

AWS ECS Container Metrics receiver reads task metadata and docker stats from [Amazon ECS Task Metadata Endpoint](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-metadata-endpoint.html), and generates resource usage metrics from them. To get a full list of available metrics, see the `Available Metrics` section below.

Note: For now, `awsecscontainermetrics` receiver works only for [ECS Task Metadata Endpoint V4](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-metadata-endpoint-v4.html). Amazon ECS tasks on Fargate that use platform version 1.4.0 or later and Amazon ECS tasks on Amazon EC2 that are running at least version 1.39.0 of the Amazon ECS container agent can utilize this receiver. For more information, see [Amazon ECS Container Agent Versions](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-agent-versions.html).


## Configuration

Example:

```yaml
receivers:
  awsecscontainermetrics:
    collection_interval: 20s
```

#### collection_interval:

This receiver collects task metadata and container stats at a fixed interval and emits metrics to the next consumer of OpenTelemetry pipeline. `collection_interval` will determine the frequency at which metrics are collected and emitted by this receiver.

default: `20s`


## Available Metrics
Following is the full list of metrics emitted by this receiver.

Task Level Metrics | Container Level Metrics | Unit 
------------ | ------------- | --------------------
ecs.task.memory.usage | container.memory.usage | Bytes
ecs.task.memory.usage.max | container.memory.usage.max | Bytes
ecs.task.memory.usage.limit | container.memory.usage.limit | Bytes
ecs.task.memory.reserved | container.memory.reserved | Megabytes
ecs.task.memory.utilized | container.memory.utilized | Megabytes
ecs.task.cpu.usage.total | container.cpu.usage.total | Nanoseconds
ecs.task.cpu.usage.kernelmode | container.cpu.usage.kernelmode | Nanoseconds
ecs.task.cpu.usage.usermode | container.cpu.usage.usermode | Nanoseconds
ecs.task.cpu.usage.system | container.cpu.usage.system | Nanoseconds
ecs.task.cpu.usage.vcpu | container.cpu.usage.vcpu | vCPU
ecs.task.cpu.cores | container.cpu.cores | Count
ecs.task.cpu.onlines | container.cpu.onlines | Count
ecs.task.cpu.reserved | container.cpu.reserved | vCPU
ecs.task.cpu.utilized | container.cpu.utilized | Percent
ecs.task.network.rate.rx	| container.network.rate.rx	| Bytes/Second
ecs.task.network.rate.tx	| container.network.rate.tx	| Bytes/Second
ecs.task.network.io.usage.rx_bytes	| container.network.io.usage.rx_bytes	| Bytes
ecs.task.network.io.usage.rx_packets	| container.network.io.usage.rx_packets	| Count
ecs.task.network.io.usage.rx_errors |	container.network.io.usage.rx_errors	| Count
ecs.task.network.io.usage.rx_dropped |	container.network.io.usage.rx_dropped	| Count
ecs.task.network.io.usage.tx_bytes | container.network.io.usage.tx_bytes	| Bytes
ecs.task.network.io.usage.tx_packets	| container.network.io.usage.tx_packets	| Count
ecs.task.network.io.usage.tx_errors	| container.network.io.usage.tx_errors	| Count
ecs.task.network.io.usage.tx_dropped	| container.network.io.usage.tx_dropped	| Count
ecs.task.storage.read_bytes | container.storage.read_bytes| Bytes
ecs.task.storage.write_bytes | container.storage.write_bytes | Bytes


## Resource Attributes and Metrics Labels
Metrics emitted by this receiver comes with a set of resource attributes. These resource attributes can be converted to metrics labels using appropriate processors/exporters (See `Full Configuration Examples` section below). Finally, these metrics labels can be set as metrics dimensions while exporting to desired destinations. Check the following table to see available resource attributes for Task and Container level metrics. Container level metrics have three additional attributes than task level metrics.

Resource Attributes for Task Level Metrics | Resource Attributes for Container Level Metrics
-------------------- | -----------------------------
aws.ecs.cluster.name | aws.ecs.cluster.name
aws.ecs.task.family  | aws.ecs.task.family
aws.ecs.task.arn     | aws.ecs.task.arn
aws.ecs.task.id      | aws.ecs.task.id
aws.ecs.task.version | aws.ecs.task.version
aws.ecs.service.name | aws.ecs.service.name
&nbsp; | container.name
&nbsp; | container.id
&nbsp; | aws.ecs.docker.name 

## Full Configuration Examples
This receiver emits 52 unique metrics. Customer may not want to send all of them to destinations. This section will show full configuration files for filtering and transforming existing metrics with different processors/exporters. 

### 1. Select specific metrics and send to Amazon CloudWatch
Followig configuration uses our `awsecscontainermetrics` receiver which provides 52 different metrics. It utilizes `filter` processor to select only 8 task level metrics and `awsemf` exporter to send them to Amazon CloudWatch. 

```yaml
receivers:
  awsecscontainermetrics:

exporters:
  awsemf:
    namespace: 'ECS/ContainerMetrics/OpenTelemetry'
    log_group_name: '/ecs/containermetrics/opentelemetry'

processors:
  filter:
    metrics:
      include:
        match_type: strict
        metric_names:
          - ecs.task.memory.utilized
          - ecs.task.memory.reserved
          - ecs.task.cpu.utilized
          - ecs.task.cpu.reserved
          - ecs.task.network.rate.rx
          - ecs.task.network.rate.tx
          - ecs.task.storage.read_bytes
          - ecs.task.storage.write_bytes

service:
  pipelines:
      metrics:
          receivers: [awsecscontainermetrics]
          processors: [filter, metricstransform]
          exporters: [awsemf]
```


### 2. Select specific metrics and rename them

We can utilize `metricstransform` processor to rename metrics before sending to destinations.

```yaml
receivers:
  awsecscontainermetrics:

exporters:
  awsemf:
    namespace: 'ECS/ContainerMetrics/OpenTelemetry'
    log_group_name: '/ecs/containermetrics/opentelemetry'

processors:
  filter:
    metrics:
      include:
        match_type: strict
        metric_names:
          - ecs.task.memory.utilized
          - ecs.task.memory.reserved
          - ecs.task.cpu.utilized
          - ecs.task.cpu.reserved
          - ecs.task.network.rate.rx
          - ecs.task.network.rate.tx
          - ecs.task.storage.read_bytes
          - ecs.task.storage.write_bytes
  metricstransform:
    transforms:
      - metric_name: ecs.task.memory.utilized
        action: update
        new_name: MemoryUtilized
      - metric_name: ecs.task.memory.reserved
        action: update
        new_name: MemoryReserved
      - metric_name: ecs.task.cpu.utilized
        action: update
        new_name: CpuUtilized
      - metric_name: ecs.task.cpu.reserved
        action: update
        new_name: CpuReserved
      - metric_name: ecs.task.network.rate.rx
        action: update
        new_name: NetworkRx
      - metric_name: ecs.task.network.rate.tx
        action: update
        new_name: NetworkTx
      - metric_name: ecs.task.storage.read_bytes
        action: update
        new_name: StorageRead
      - metric_name: ecs.task.storage.write_bytes
        action: update
        new_name: StorageWrite

service:
  pipelines:
      metrics:
          receivers: [awsecscontainermetrics]
          processors: [filter, metricstransform]
          exporters: [awsemf]
```