# InfluxDB Exporter

This exporter supports sending tracing, metrics, and logging data to [InfluxDB](https://www.influxdata.com/products/).

Supported pipeline types: traces, metrics, logs

## Configuration

The following configuration options are supported:

* `org` (Required): Org is the InfluxDB organization name of the destination bucket.
* `bucket` (Required): Bucket is the InfluxDB bucket name that telemetry will be written to.
* `protocol` (Optional): Protocol is the method used to deliver metrics to InfluxDB. Must be one of `line-protocol`, `grpc`. Defaults to `line-protocol`.
* `line_protocol_server_url` (Required): LineProtocolServerUrl is the destination for Protocol `line-protocol`.
* `line_protocol_auth_token`: LineProtocolAuthToken is the authentication token for Protocol `line-protocol`.
* `timeout` (Optional): Timeout is the timeout for every attempt to send data to InfluxDB. Defaults to `5s`.


Example:
```yaml
exporters:
  influxdb:
    org: "my-org"
    bucket: "my-bucket"
    protocol: "line-protocol"
    line_protocol_server_url: "http://localhost:8086"
    line_protocol_auth_token: "my-token"
    timeout: 10s
```

The full list of settings exposed for this exporter are documented [here](config.go).

## Schema

InfluxDB/IOx is an open-source time series storage engine, capable of storing and querying large (and small) time series datasets.
The schema used by this OpenTelemetry exporter is optimized for InfluxDB/IOx performance.

### Definitions

An OpenTelemetry ["Signal"](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/overview.md#opentelemetry-client-architecture) is general term for instances of categories of telemetry.
For example:
- Trace spans
- Metrics
- Logs

An OpenTelemetry Signal ["Attribute"](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/common/common.md#attributes) to be a key-value pair whose key is a non-null, non-empty string, and whose value is not null and of type string, boolean, float64, int64, or homogeneous arrays of the previously mentioned primitives.
Attribute names should follow the [naming guidelines](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/common/attribute-and-label-naming.md).
For example:
- `service.name`
- `host.name`
- `cloud.region`
- `org_id`
- `env`

An OpenTelemetry Signal Label is similar to a Signal Attribute.

An OpenTelemetry Signal ["Resource"](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/resource/sdk.md) Attribute is "an immutable representation of the entity producing telemetry".
In other words, the entity that generated the Signal.
For example:
- Host machine or VM
- Kubernetes node, pod, and container
- Application

An OpenTelemetry Signal ["Instrumentation Library"](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/overview.md#instrumentation-libraries) Attribute is "a library that enables OpenTelemetry observability for another library".
In other words, the library used by the host application to generate the Signal.
For example:
- The Go OpenTelemetry implementation
- The Rust OpenTelemetry implementation

Signal Attributes are also defined by the application emitting the Signal.

Signals also have properties that are not free-form Attributes or Labels.
These properties distinguish the OpenTelemetry Signal types.
For example:
- trace ID, span ID, parent span ID
- metric name
- log severity, log span ID

### InfluxDB Conversion from OpenTelemetry

Spans are stored in measurement `spans`.
Metric points are assigned measurement from the `Metric.name` protocol buffer field.
Logs are stored in measurement `logs`.

This exporter converts Attributes to InfluxDB fields, verbatim.
In the case that application Attribute keys conflict with Resource or Instrumentation Library Attribute keys, the application loses.

The exporter converts other Signal properties to fields with key names borrowed from [OTLP protocol buffer](https://github.com/open-telemetry/opentelemetry-proto) messages.
For example:
- `Span.start_time_unix_nano` (type fixed64) -> InfluxDB line protocol timestamp
- `Span.trace_id` (type bytes) -> `trace_id` as hexadecimal string
- `Span.name` (type string) -> `name`
- `Span.end_time_unix_nano` (type fixed64) -> `end_time_unix_nano` as uint64
- `Metric.time_unix_nano` (type fixed64) -> InfluxDB line protocol timestamp
- `LogRecord.time_unix_nano` (type fixed64) -> InfluxDB line protocol timestamp
  - This is an optional field in the OpenTelemetry data model, but required in InfluxDB.
  - This exporter drops the LogRecord if this field value is not set.
- `LogRecord.severity_number` (type enum) -> `severity_number` as int32
- `LogRecord.body` (type opentelemetry.proto.common.v1.AnyValue) -> `body` as string

Some exceptions to the above exist.
For example:
- `Span.events` (type repeated message) -> JSON string
- `Span.links` (type repeated message) JSON string
  - TODO consider a links measurement for reverse link queries
- `Metric.description` is ignored
- `Metric.unit` is ignored
  - It is currently ignored in other exporters as well
- `Metric` values are assigned field key `gauge`, `counter`, `count`, `sum`, or `inf`
  - Metric conversion follows Prometheus conventions for future backward compatibility
- `Metric` instances with aggregation temporality type `DELTA` are dropped
  - The Prometheus exporter also drops these
- `LogRecord.flags` is ignored
  - This is an enum with no values defines yet

### Example Line Protocol

#### Tracing Spans
```
spans end_time_unix_nano="2021-02-19 20:50:25.6893952 +0000 UTC",instrumentation_library_name="tracegen",kind="SPAN_KIND_INTERNAL",name="okey-dokey",net.peer.ip="1.2.3.4",parent_span_id="d5270e78d85f570f",peer.service="tracegen-client",service.name="tracegen",span.kind="server",span_id="4c28227be6a010e1",status_code="STATUS_CODE_OK",trace_id="7d4854815225332c9834e6dbf85b9380" 1613767825689169000
spans end_time_unix_nano="2021-02-19 20:50:25.6893952 +0000 UTC",instrumentation_library_name="tracegen",kind="SPAN_KIND_INTERNAL",name="lets-go",net.peer.ip="1.2.3.4",peer.service="tracegen-server",service.name="tracegen",span.kind="client",span_id="d5270e78d85f570f",status_code="STATUS_CODE_OK",trace_id="7d4854815225332c9834e6dbf85b9380" 1613767825689135000
spans end_time_unix_nano="2021-02-19 20:50:25.6895667 +0000 UTC",instrumentation_library_name="tracegen",kind="SPAN_KIND_INTERNAL",name="okey-dokey",net.peer.ip="1.2.3.4",parent_span_id="b57e98af78c3399b",peer.service="tracegen-client",service.name="tracegen",span.kind="server",span_id="a0643a156d7f9f7f",status_code="STATUS_CODE_OK",trace_id="fd6b8bb5965e726c94978c644962cdc8" 1613767825689388000
spans end_time_unix_nano="2021-02-19 20:50:25.6895667 +0000 UTC",instrumentation_library_name="tracegen",kind="SPAN_KIND_INTERNAL",name="lets-go",net.peer.ip="1.2.3.4",peer.service="tracegen-server",service.name="tracegen",span.kind="client",span_id="b57e98af78c3399b",status_code="STATUS_CODE_OK",trace_id="fd6b8bb5965e726c94978c644962cdc8" 1613767825689303300
spans end_time_unix_nano="2021-02-19 20:50:25.6896741 +0000 UTC",instrumentation_library_name="tracegen",kind="SPAN_KIND_INTERNAL",name="okey-dokey",net.peer.ip="1.2.3.4",parent_span_id="6a8e6a0edcc1c966",peer.service="tracegen-client",service.name="tracegen",span.kind="server",span_id="d68f7f3b41eb8075",status_code="STATUS_CODE_OK",trace_id="651dadde186b7834c52b13a28fc27bea" 1613767825689480300
```

#### Metrics
```
avalanche_metric_mmmmm_0_71 cycle_id="0",gauge=29,host.name="generate-metrics-avalanche",label_key_kkkkk_0="label_val_vvvvv_0",label_key_kkkkk_1="label_val_vvvvv_1",label_key_kkkkk_2="label_val_vvvvv_2",label_key_kkkkk_3="label_val_vvvvv_3",label_key_kkkkk_4="label_val_vvvvv_4",label_key_kkkkk_5="label_val_vvvvv_5",label_key_kkkkk_6="label_val_vvvvv_6",label_key_kkkkk_7="label_val_vvvvv_7",label_key_kkkkk_8="label_val_vvvvv_8",label_key_kkkkk_9="label_val_vvvvv_9",port="9090",scheme="http",series_id="3",service.name="otel-collector" 1613772311130000000
avalanche_metric_mmmmm_0_71 cycle_id="0",gauge=16,host.name="generate-metrics-avalanche",label_key_kkkkk_0="label_val_vvvvv_0",label_key_kkkkk_1="label_val_vvvvv_1",label_key_kkkkk_2="label_val_vvvvv_2",label_key_kkkkk_3="label_val_vvvvv_3",label_key_kkkkk_4="label_val_vvvvv_4",label_key_kkkkk_5="label_val_vvvvv_5",label_key_kkkkk_6="label_val_vvvvv_6",label_key_kkkkk_7="label_val_vvvvv_7",label_key_kkkkk_8="label_val_vvvvv_8",label_key_kkkkk_9="label_val_vvvvv_9",port="9090",scheme="http",series_id="4",service.name="otel-collector" 1613772311130000000
avalanche_metric_mmmmm_0_71 cycle_id="0",gauge=22,host.name="generate-metrics-avalanche",label_key_kkkkk_0="label_val_vvvvv_0",label_key_kkkkk_1="label_val_vvvvv_1",label_key_kkkkk_2="label_val_vvvvv_2",label_key_kkkkk_3="label_val_vvvvv_3",label_key_kkkkk_4="label_val_vvvvv_4",label_key_kkkkk_5="label_val_vvvvv_5",label_key_kkkkk_6="label_val_vvvvv_6",label_key_kkkkk_7="label_val_vvvvv_7",label_key_kkkkk_8="label_val_vvvvv_8",label_key_kkkkk_9="label_val_vvvvv_9",port="9090",scheme="http",series_id="5",service.name="otel-collector" 1613772311130000000
avalanche_metric_mmmmm_0_71 cycle_id="0",gauge=90,host.name="generate-metrics-avalanche",label_key_kkkkk_0="label_val_vvvvv_0",label_key_kkkkk_1="label_val_vvvvv_1",label_key_kkkkk_2="label_val_vvvvv_2",label_key_kkkkk_3="label_val_vvvvv_3",label_key_kkkkk_4="label_val_vvvvv_4",label_key_kkkkk_5="label_val_vvvvv_5",label_key_kkkkk_6="label_val_vvvvv_6",label_key_kkkkk_7="label_val_vvvvv_7",label_key_kkkkk_8="label_val_vvvvv_8",label_key_kkkkk_9="label_val_vvvvv_9",port="9090",scheme="http",series_id="6",service.name="otel-collector" 1613772311130000000
avalanche_metric_mmmmm_0_71 cycle_id="0",gauge=51,host.name="generate-metrics-avalanche",label_key_kkkkk_0="label_val_vvvvv_0",label_key_kkkkk_1="label_val_vvvvv_1",label_key_kkkkk_2="label_val_vvvvv_2",label_key_kkkkk_3="label_val_vvvvv_3",label_key_kkkkk_4="label_val_vvvvv_4",label_key_kkkkk_5="label_val_vvvvv_5",label_key_kkkkk_6="label_val_vvvvv_6",label_key_kkkkk_7="label_val_vvvvv_7",label_key_kkkkk_8="label_val_vvvvv_8",label_key_kkkkk_9="label_val_vvvvv_9",port="9090",scheme="http",series_id="7",service.name="otel-collector" 1613772311130000000
```

#### Logs
```
logs fluent.tag="fluent.info",pid=18i,ppid=9i,worker=0i 1613769568895331700
logs fluent.tag="fluent.debug",instance=1720i,queue_size=0i,stage_size=0i 1613769568895697200
logs fluent.tag="fluent.info",worker=0i 1613769568896515100
```
