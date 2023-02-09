# Google Pubsub Receiver

| Status                   |                       |
| ------------------------ |-----------------------|
| Stability                | [beta]                |
| Supported pipeline types | traces, logs, metrics |
| Distributions            | none                  |

> ⚠️ This is a community-provided module. It has been developed and extensively tested at Collibra, but it is not officially supported by GCP.
 
This receiver gets OTLP messages from a Google Cloud [Pubsub](https://cloud.google.com/pubsub) subscription.

The following configuration options are supported:

* `mode` (Optional): The mode that the receiver is in, either `push` or `pull` is supported. The default is `pull`.
* `project` (Optional): The Google Cloud Project of the client connects to. Only used in `pull` mode.
* `subscription` (Required in pull mode): The subscription name to receive OTLP data from. The subscription name  should be a 
  fully qualified resource name (eg: `projects/otel-project/subscriptions/otlp`). Only used in `pull` mode.
* `encoding` (Optional): The encoding that will be used to received data from the subscription. This can either be
  `otlp_proto_trace`, `otlp_proto_metric`, `otlp_proto_log`, or `raw_text` (see `encoding`).  This will only be used as 
  a fallback, when no `content-type` attribute is present.
* `compression` (Optional): The compression that will be used on received data from the subscription. When set it can 
  only be `gzip`. This will only be used as a fallback, when no `content-encoding` attribute is present.
* `endpoint` (Optional): Override the default Pubsub Endpoint, useful when connecting to the PubSub emulator instance
  or switching between [global and regional service endpoints](https://cloud.google.com/pubsub/docs/reference/service_apis_overview#service_endpoints). Only used in `pull` mode.
* `insecure` (Optional): allows performing “insecure” SSL connections and transfers, useful when connecting to a local
   emulator instance. Only has effect if Endpoint is not "", and in `pull` mode.
* `push`
  * `path` (Optional): The path the receiver listens on for Pub/Sub messages (default is `/`).
  * `endpoint` (Required in push mode): The endpoint the receiver listens on for Pub/Sub messages.

```yaml
receivers:
  googlecloudpubsub:
    project: otel-project
    subscription: projects/otel-project/subscriptions/otlp-logs
    encoding: raw_json
```

The full list of settings exposed for this receiver are documented [here](./config.go).

## Modes

The receiver supports two modes: `push` and `pull`. If you don't explicitly set the mode in the config, the default is `pull`.

| mode | description                                                                                                                                                                              |
|------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| pull | The receivers pulls messages from a subscription.                                                                                                                                        |
| push | The receivers is set to handle [Push subscriptions](https://cloud.google.com/pubsub/docs/push). _Known limitation_: You can not define different receivers that listen on the same port. |

## Encoding

You should not need to set the encoding of the subscription as the receiver will try to discover the type of the data
by looking at the `ce-type` and `content-type` attributes of the message. Only when those attributes are not set 
must the `encoding` field in the configuration be set. 

| ce-type]                          | ce-datacontenttype   | encoding          | description                 |
|-----------------------------------|----------------------|-------------------|-----------------------------|
| org.opentelemetry.otlp.traces.v1  | application/protobuf |                   | Decode OTLP trace message   |
| org.opentelemetry.otlp.metrics.v1 | application/protobuf |                   | Decode OTLP metric message  |
| org.opentelemetry.otlp.logs.v1    | application/json     |                   | Decode OTLP log message     |
| -                                 | -                    | otlp_proto_trace  | Decode OTLP trace message   |
| -                                 | -                    | otlp_proto_metric | Decode OTLP trace message   |
| -                                 | -                    | otlp_proto_log    | Decode OTLP trace message   |
| -                                 | -                    | raw_text          | Wrap in an OTLP log message |

When the `encoding` configuration is set, the attributes on the message are ignored.

The receiver can be used for ingesting arbitrary text message on a Pubsub subscription and wrap them in OTLP Log
message, making it a convenient way to ingest log lines from Pubsub.

## Pubsub subscription

The Google Cloud [Pubsub](https://cloud.google.com/pubsub) receiver doesn't automatically create subscriptions, 
it expects the subscription to be created upfront. Security wise it's best to give the collector its own 
service account and give the subscription `Pub/Sub Subscriber` permission.

### Push mode

The receiver also handles [Push subscriptions](https://cloud.google.com/pubsub/docs/push). The receiver needs to be exposed to the internet so Google Pub/Sub can reach your receiver, but on the other hand, there is no need for having access to a service account key.

```yaml
receivers:
  googlecloudpubsub:
    mode: push
    push:
      path: /
      endpoint: "0.0.0.0:4242"
```

Pub/Sub requires your endpoint to have a valid [SSL certificate signed by a certificate authority](https://cloud.google.com/pubsub/docs/push#properties_of_a_push_subscription). This can either be done through a proxy in front or configured explicitly in the configuration; the receiver supports the standard [HTTP endpoint attributes](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/configtls/README.md).

### Filtering

When the messages on the subscription are accompanied by the correct attributes and you only need a specific
type in your pipeline, the messages can be [filtered](https://cloud.google.com/pubsub/docs/filtering) on the 
subscription saving on egress fees.

An example of filtering on trace message only: 
```
attributes.ce-type = "org.opentelemetry.otlp.traces.v1"
AND
attributes.content-type = "application/protobuf"
```

[beta]: https://github.com/open-telemetry/opentelemetry-collector#beta
