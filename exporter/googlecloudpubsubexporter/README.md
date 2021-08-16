# Google Cloud Pubsub Exporter

> ⚠️ This is a community-provided module. It has been developed and extensively tested at Collibra, but it is not officially supported by GCP.

This exporter sends OTLP messages to a Google Cloud [Pubsub](https://cloud.google.com/pubsub) topic.

The following configuration options are supported:

* `project` (Optional): The Google Cloud Project of the topics.
* `topic` (Required): The topic name to receive OTLP data over. The topic name should be a fully qualified resource
  name (eg: `projects/otel-project/topics/otlp`).

```yaml
exporters:
  googlecloudpubsub:
    project: my-project
    topic: otlp-traces
```

## Pubsub topic

The Google Cloud [Pubsub](https://cloud.google.com/pubsub) export doesn't automatic create topics, it expects the topic
to be created upfront. Security wise it's best to give the collector its own service account and give the
topic `Pub/Sub Publisher` permission.

## Messages

The message published on the topic are [CloudEvent](https://cloudevents.io/) compliance and uses the binary content mode
defined in the
[Google Cloud Pub/Sub Protocol Binding for CloudEvents](https://github.com/google/knative-gcp/blob/main/docs/spec/pubsub-protocol-binding.md#31-binary-content-mode)
.

The data field is either a `ExportTraceServiceRequest`, `ExportMetricsServiceRequest` or `ExportLogsServiceRequest` for 
traces, metrics or logs respectively.  Each message is accompanied with the following attributes:

| attributes | description |
| --- | --- |
| ce-specversion | Follow version `1.0` of the CloudEvent spec |
| ce-source | The source is this `/opentelemetry/collector/googlecloudpubsub/v0.27.0` exporter |
| ce-id | a random `UUID` to uniquely define the message |
| ce-type | depending on the data `org.opentelemetry.otlp.traces.v1`, `org.opentelemetry.otlp.metrics.v1` or `org.opentelemetry.otlp.logs.v1` |
| ce-datacontenttype | the content type is `application/x-protobuf` | 
