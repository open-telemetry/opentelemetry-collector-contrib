# AWS ECS Container Metrics Receiver

***Status:***
This receiver is under development and not recommended for production usage.

### Overview

This receiver reads task metadata and docker stats from [Amazon ECS Task Metadata Endpoint](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-metadata-endpoint.html).

### Config

An example config,

```yaml
  awsecscontainermetrics:
    collection_interval: 20s
```

#### collection_interval

This receiver collects task metadata and container stats at a fixed interval and emits metrics to the next consumer of OpenTelemetry pipeline. `collection_interval` will determine the frequency at which metrics are collected and emitted by this receiver.

default: `20s`