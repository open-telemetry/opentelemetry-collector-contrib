# StatsD Receiver

StatsD receiver for ingesting StatsD messages(https://github.com/statsd/statsd/blob/master/docs/metric_types.md) into the OpenTelemetry Collector.

Supported pipeline types: metrics

Use case: it does not support horizontal pool of collectors. Desired work case is that customers use the receiver as an agent with a single input at the same time.

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
Counter(transferred to int):
- statsdTestMetric1:3000|c|#mykey:myvalue
statsdTestMetric1:4000|c|#mykey:myvalue
(get the value after incrementation: 7000)
- statsdTestMetric1:3000|c|#mykey:myvalue
statsdTestMetric1:20|c|@0.25|#mykey:myvalue
(get the value after incrementation with sample rate: 3000+20/0.25=3080)

When the receiver receives valid sample rate (greater than 0 and less than 1), we covert the count value to float, divide by the sample rate and then covert back to integer.

The official [doc](https://github.com/statsd/statsd/blob/master/docs/metric_types.md#counting) does not support negative counter, we follow this pattern at this time. There are some requests for negative counters, we need to ake a look if we want to support later. For example:
https://github.com/influxdata/telegraf/issues/1898
https://thenewstack.io/collecting-metrics-using-statsd-a-standard-for-real-time-monitoring/
https://docs.datadoghq.com/developers/metrics/dogstatsd_metrics_submission/#count

Gauge(transferred to double):
- statsdTestMetric1:500|g|#mykey:myvalue
statsdTestMetric1:400|g|#mykey:myvalue
(get the latest value: 400)
- statsdTestMetric1:500|g|#mykey:myvalue
statsdTestMetric1:+2|g|#mykey:myvalue
statsdTestMetric1:-1|g|#mykey:myvalue
(get the value after calculation: 501)

## Metrics

General format is:

`<name>:<value>|<type>|@<sample-rate>|#<tag1-key>:<tag1-value>,<tag2-k/v>`

### Counter

`<name>:<value>|c|@<sample-rate>|#<tag1-key>:<tag1-value>`

It supports sample rate.
TODO: Use OTLP type(Sum data points, with AggregationTemporality=Delta and Monotonic=False) for transferred data types (now we are using OpenCensus types).
TODO: Need to change the implementation part for sample rate after OTLP supports sample rate as a parameter later.


### Gauge

`<name>:<value>|g|@<sample-rate>|#<tag1-key>:<tag1-value>`

TODO: Use OTLP type for transferred data types (now we are using OpenCensus types).

### Timer

TODO: add support for timer and histogram.

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
