# Pulsar Exporter

Pulsar exporter exports traces/metrics/logs to pulsar. Message payload encoding is configurable.
 
The following settings are required:
- `topic`: The name of the pulsar topic to export to, recommended for otlp_traces for traces, otlp_metrics for metrics, otlp_logs for logs.

The following settings can be optionally configured:
- `url` (default = pulsar://localhost:6650): The url of pulsar cluster
- `encoding` (default = otlp_proto): The encoding of the traces sent to pulsar. All available encodings:
  - `otlp_proto`: payload is Protobuf serialized from `ExportTraceServiceRequest` if set as a traces exporter or `ExportMetricsServiceRequest` for metrics.
  - The following encodings are valid *only* for **traces**.
    - `jaeger_proto`: the payload is serialized to a single Jaeger proto `Span`, and keyed by TraceID.
    - `jaeger_json`: the payload is serialized to a single Jaeger JSON Span using `jsonpb`, and keyed by TraceID.
- `auth`
  - `name` pulsar auth name, more detail need ref pulsar sdk.
  - `params` pulsar auth params map, more detail need ref pulsar sdk.
    - `token`: pulsar jwt token
- `connection_timeout`: (default = 10s) the timeout of connect to pulsar cluster
- `operation_timeout`: (default = ${pulsar sdk default}}) sdk operation timeout
- `max_connections_per_broker`: (default = ${pulsar sdk default}}) max connections per broker
- `send_timeout`: (default = ${pulsar sdk default}}) producer send message timeout
- `max_pending_messages`: (default = 10000) set the max size of the queue holding the messages pending to receive an acknowledgment from the broker.
- `compression_type`: (default = 0) set the compression type for the producer.
- `compression_level`: (default = 0) define the desired compression level.
- `batching_max_publish_delay`: (default = 10ms) set the time period within which the messages sent will be batched.
- `batching_max_messages`:  (default = 1000) set the maximum number of messages permitted in a batch.
- `batching_max_size`: (default = 1024*1024(1MB)) sets the maximum number of bytes permitted in a batch.
- `attribute_as_send_key`: (default = '') attribute name as producer send key for hash balance.

Example configuration:

```yaml
exporters:
  pulsar/traces:
    topic: otlp_traces
    url: pulsar://localhost:6650
  pulsar/metrics:
    topic: otlp_metrics
    url: pulsar://localhost:6650
  pulsar/logs:
    topic: otlp_logs
    url: pulsar://localhost:6650
  pulsar/allsettings:
    url: pulsar://localhost:6650
    connection_timeout: 10s
    operation_timeout: 10s
    auth:
      name: token
      params: '{"token":"123"}'
    max_connections_per_broker: 10
    topic: otlp_traces
    encoding: otlp_proto
    send_timeout: 1s
    max_pending_messages: 1000
    compression_type: 1
    compression_level: 1
    batching_max_publish_delay: 100ms
    batching_max_messages: 1024
    batching_max_size: 1048576
    attribute_as_send_key: service.name
  pulsar:
    topic: otlp_traces
    url: pulsar://localhost:6650
processors:
  nop:

receivers:
  nop:

service:
  pipelines:
    traces/producer:
      receivers: [ nop ]
      processors: [ nop ]
      exporters: [ pulsar/traces ]
    traces/consumer:
      receivers: [ pulsar/traces ]
      processors: [ nop ]
      exporters: [ nop ]
    metrics:
      receivers: [ nop ]
      processors: [ nop ]
      exporters: [ pulsar/metrics ]
    logs:
      receivers: [ nop ]
      processors: [ nop ]
      exporters: [ pulsar/logs ]
```
