# StatsD Receiver

StatsD receiver for ingesting StatsD messages(https://github.com/statsd/statsd/blob/master/docs/metric_types.md) into the OpenTelemetry Collector.

Supported pipeline types: metrics

> :construction: This receiver is currently in **BETA**.

## Configuration

The following settings are required:

- `endpoint` (default = `localhost:8125`): Address and port to listen on.


The Following settings are optional:

- `aggregation_interval: 70s`(default value is 60s): The aggregation time that the receiver aggregates the metrics (similar to the flush interval in StatsD server)

Example:

```yaml
receivers:
  statsd:
  statsd/2:
    endpoint: "localhost:8127"
    aggregation_interval: 70s
```

The full list of settings exposed for this receiver are documented [here](./config.go)
with detailed sample configurations [here](./testdata/config.yaml).

## Aggregation

Aggregation is done in statsD receiver. The default aggregation interval is 60s. The receiver only aggregates the metrics with the same metric name, metric type, label keys and label values. After each aggregation interval, the receiver will send all metrics (after aggregation) in this aggregation interval to the following workflow.

It supports:

Gauge(transferred to double):
- statsdTestMetric1:500|g|#mykey:myvalue
statsdTestMetric1:400|g|#mykey:myvalue
(get the latest value: 400)
- statsdTestMetric1:500|g|#mykey:myvalue
statsdTestMetric1:+2|g|#mykey:myvalue
statsdTestMetric1:-1|g|#mykey:myvalue
(get the value after calculation: 501)

Counter(transferred to int):
- statsdTestMetric1:3000|c|#mykey:myvalue
statsdTestMetric1:4000|c|#mykey:myvalue
(get the value after incrementation: 7000)
- statsdTestMetric1:3000|c|#mykey:myvalue
statsdTestMetric1:20|c|@0.8|#mykey:myvalue
(get the value after incrementation with sample rate: 3000+20/0.8=3025)
## Metrics

General format is:

`<name>:<value>|<type>|@<sample-rate>|#<tag1-key>:<tag1-value>,<tag2-k/v>`

### Counter

`<name>:<value>|c|@<sample-rate>|#<tag1-key>:<tag1-value>`

it supports sample rate

### Gauge

`<name>:<value>|g|@<sample-rate>|#<tag1-key>:<tag1-value>`

### Timer

TODO: add support for timer

## Testing

### Full sample collector config

```yaml
receivers:
  statsd:
    endpoint: "localhost:8125" # default
    aggregation_interval: 60s  # default

exporters:
  file:
    path: ./test.json

service:
  pipelines:
    metrics:
     receivers: [statsd]
     exporters: [file]
```

### Send StatsD message into the receiver

A simple way to send a metric to `localhost:8125`:

`echo "test.metric:42|c|#myKey:myVal" | nc -w 1 -u localhost 8125`
