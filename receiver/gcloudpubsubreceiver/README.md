# Google Pubsub Receiver

This receiver gets OTLP messages from a Google Cloud [Pubsub](https://cloud.google.com/pubsub) subscription.

The following configuration options are supported:

* `project` (Required): The Google Cloud Project of the topics.
* `validate_existence`(Optional): Checks the existence of the subscription, but this requires admin permissions to validate
  the existence.
* `traces_subscription` (Optional): The subscription name to receive OTLP trace data from, this is the name within the project.
* `metrics_subscription` (Optional): The subscription name to receive OTLP metric data from, this is the name within the project.
* `logs_subscription` (Optional): The subscription name to receive OTLP log data over, from is the name within the project.

```yaml
receivers:
  gcloudpubsub:
    project: my-project
    validate_existence: false
    traces_subscription: otlp-traces-subscription
    metrics_subscription: otlp-metrics-subscription
    logs_subscription: otlp-logs-subscription
```

