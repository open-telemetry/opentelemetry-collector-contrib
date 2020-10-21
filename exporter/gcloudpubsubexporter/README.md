# Google Pubsub Exporter

This exporter sends OTLP messages to a Google Cloud [Pubsub](https://cloud.google.com/pubsub) topic.

The following configuration options are supported:

* `project` (Required): The Google Cloud Project of the topics.
* `validate_existence`(Optional): Checks the existence of the topic, but this requires admin permissions to validate 
  the existence.
* `traces_topic` (Optional): The topic name to send OTLP trace data over, this is the name within the project.
* `metrics_topic` (Optional): The topic name to send OTLP metric data over, this is the name within the project.
* `logs_topic` (Optional): The topic name to send OTLP log data over, this is the name within the project.

```yaml
exporters:
  gcloudpubsub:
    project: my-project
    validate_existence: false
    traces_topic: otlp-traces
    metrics_topic: otlp-metrics
    logs_topic: otlp-logs
```

