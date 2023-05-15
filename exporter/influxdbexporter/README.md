# InfluxDB Exporter

| Status                   |                       |
| ------------------------ |-----------------------|
| Stability                | [beta]                |
| Supported pipeline types | traces, logs, metrics |
| Distributions            | [contrib]             |

This exporter supports sending tracing, metrics, and logging data to [InfluxDB](https://www.influxdata.com/products/).

## Configuration

The following configuration options are supported:

* `endpoint` (required) HTTP/S destination for line protocol
  - if path is set to root (/) or is unspecified, it will be changed to /api/v2/write.
* `timeout` (default = 5s) Timeout for requests
* `headers`: (optional) additional headers attached to each HTTP request
  - header `User-Agent` is `OpenTelemetry -> Influx` by default
  - if `token` (below) is set, then header `Authorization` will overridden with the given token
* `org` (required) Name of InfluxDB organization that owns the destination bucket
* `bucket` (required) name of InfluxDB bucket to which signals will be written
* `token` (optional) The authentication token for InfluxDB
* `v1_compatibility` (optional) Options for exporting to InfluxDB v1.x
  * `enabled` (optional) Use InfluxDB v1.x API if enabled
  * `db` (required if enabled) Name of the InfluxDB database to which signals will be written
  * `username` (optional) Basic auth username for authenticating with InfluxDB v1.x
  * `password` (optional) Basic auth password for authenticating with InfluxDB v1.x
* `metrics_schema` (default = telegraf-prometheus-v1) The chosen metrics schema to write; must be one of:
  * `telegraf-prometheus-v1`
  * `telegraf-prometheus-v2`
* `sending_queue` [details here](https://github.com/open-telemetry/opentelemetry-collector/blob/v0.25.0/exporter/exporterhelper/README.md#configuration)
  * `enabled` (default = true)
  * `num_consumers` (default = 10) The number of consumers from the queue
  * `queue_size` (default = 1000) Maximum number of batches allowed in queue at a given time
* `retry_on_failure` [details here](https://github.com/open-telemetry/opentelemetry-collector/blob/v0.25.0/exporter/exporterhelper/README.md#configuration)
  * `enabled` (default = true)
  * `initial_interval` (default = 5s) Time to wait after the first failure before retrying
  * `max_interval` (default = 30s) Upper bound on backoff interval
  * `max_elapsed_time` (default = 120s) Maximum amount of time (including retries) spent trying to send a request/batch

The full list of settings exposed for this exporter are documented in [config.go](config.go).

Example:
```yaml
exporters:
  influxdb:
    endpoint: http://localhost:8080
    timeout: 500ms
    org: my-org
    bucket: my-bucket
    token: my-token
    metrics_schema: telegraf-prometheus-v1

    sending_queue:
      enabled: true
      num_consumers: 3
      queue_size: 10

    retry_on_failure:
      enabled: true
      initial_interval: 1s
      max_interval: 3s
      max_elapsed_time: 10s
```

## Definitions

[InfluxDB](https://www.influxdata.com/products/influxdb/) is an open-source time series database.

[Telegraf](https://www.influxdata.com/time-series-platform/telegraf/) is an open-source metrics agent, similar to the OpenTelemetry Collector.
Telegraf has [hundreds of plugins](https://www.influxdata.com/products/integrations/?_integrations_dropdown=telegraf-plugins).

[Line protocol](https://docs.influxdata.com/influxdb/v2.0/reference/syntax/line-protocol/) is a textual HTTP payload format used to move metrics between Telegraf agents and InfluxDB instances.

## Schema

The OpenTelemetry->InfluxDB conversion [schema](https://github.com/influxdata/influxdb-observability/blob/main/docs/index.md) and [implementation](https://github.com/influxdata/influxdb-observability/tree/main/otel2influx) are hosted at https://github.com/influxdata/influxdb-observability .

Spans are stored in measurement `spans`.
Metric points through `metrics_schema=telegraf-prometheus-v1` are assigned measurement from the OTel field `Metric.name`.
Metric points through `metrics_schema=telegraf-prometheus-v2` are stored in measurement `prometheus`.
Logs are stored in measurement `logs`.

### Example: Tracing Spans
```
spans end_time_unix_nano="2021-02-19 20:50:25.6893952 +0000 UTC",instrumentation_library_name="tracegen",kind="SPAN_KIND_INTERNAL",name="okey-dokey",net.peer.ip="1.2.3.4",parent_span_id="d5270e78d85f570f",peer.service="tracegen-client",service.name="tracegen",span.kind="server",span_id="4c28227be6a010e1",status_code="STATUS_CODE_OK",trace_id="7d4854815225332c9834e6dbf85b9380" 1613767825689169000
spans end_time_unix_nano="2021-02-19 20:50:25.6893952 +0000 UTC",instrumentation_library_name="tracegen",kind="SPAN_KIND_INTERNAL",name="lets-go",net.peer.ip="1.2.3.4",peer.service="tracegen-server",service.name="tracegen",span.kind="client",span_id="d5270e78d85f570f",status_code="STATUS_CODE_OK",trace_id="7d4854815225332c9834e6dbf85b9380" 1613767825689135000
spans end_time_unix_nano="2021-02-19 20:50:25.6895667 +0000 UTC",instrumentation_library_name="tracegen",kind="SPAN_KIND_INTERNAL",name="okey-dokey",net.peer.ip="1.2.3.4",parent_span_id="b57e98af78c3399b",peer.service="tracegen-client",service.name="tracegen",span.kind="server",span_id="a0643a156d7f9f7f",status_code="STATUS_CODE_OK",trace_id="fd6b8bb5965e726c94978c644962cdc8" 1613767825689388000
spans end_time_unix_nano="2021-02-19 20:50:25.6895667 +0000 UTC",instrumentation_library_name="tracegen",kind="SPAN_KIND_INTERNAL",name="lets-go",net.peer.ip="1.2.3.4",peer.service="tracegen-server",service.name="tracegen",span.kind="client",span_id="b57e98af78c3399b",status_code="STATUS_CODE_OK",trace_id="fd6b8bb5965e726c94978c644962cdc8" 1613767825689303300
spans end_time_unix_nano="2021-02-19 20:50:25.6896741 +0000 UTC",instrumentation_library_name="tracegen",kind="SPAN_KIND_INTERNAL",name="okey-dokey",net.peer.ip="1.2.3.4",parent_span_id="6a8e6a0edcc1c966",peer.service="tracegen-client",service.name="tracegen",span.kind="server",span_id="d68f7f3b41eb8075",status_code="STATUS_CODE_OK",trace_id="651dadde186b7834c52b13a28fc27bea" 1613767825689480300
```

### Example: Metrics - `telegraf-prometheus-v1`
```
cpu_temp,foo=bar gauge=87.332
http_requests_total,method=post,code=200 counter=1027
http_requests_total,method=post,code=400 counter=3
http_request_duration_seconds 0.05=24054,0.1=33444,0.2=100392,0.5=129389,1=133988,sum=53423,count=144320,min=0,max=10
rpc_duration_seconds 0.01=3102,0.05=3272,0.5=4773,0.9=9001,0.99=76656,sum=1.7560473e+07,count=2693
```

### Example: Metrics - `telegraf-prometheus-v2`
```
prometheus,foo=bar cpu_temp=87.332
prometheus,method=post,code=200 http_requests_total=1027
prometheus,method=post,code=400 http_requests_total=3
prometheus,le=0.05 http_request_duration_seconds_bucket=24054
prometheus,le=0.1  http_request_duration_seconds_bucket=33444
prometheus,le=0.2  http_request_duration_seconds_bucket=100392
prometheus,le=0.5  http_request_duration_seconds_bucket=129389
prometheus,le=1    http_request_duration_seconds_bucket=133988
prometheus         http_request_duration_seconds_count=144320,http_request_duration_seconds_sum=53423,http_request_duration_seconds_min=0,http_request_duration_seconds_max=100
prometheus,quantile=0.01 rpc_duration_seconds=3102
prometheus,quantile=0.05 rpc_duration_seconds=3272
prometheus,quantile=0.5  rpc_duration_seconds=4773
prometheus,quantile=0.9  rpc_duration_seconds=9001
prometheus,quantile=0.99 rpc_duration_seconds=76656
prometheus               rpc_duration_seconds_count=1.7560473e+07,rpc_duration_seconds_sum=2693
```

### Example: Logs
```
logs fluent.tag="fluent.info",pid=18i,ppid=9i,worker=0i 1613769568895331700
logs fluent.tag="fluent.debug",instance=1720i,queue_size=0i,stage_size=0i 1613769568895697200
logs fluent.tag="fluent.info",worker=0i 1613769568896515100
```

[beta]:https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
