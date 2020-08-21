# StatsD Receiver

StatsD receiver for ingesting StatsD messages into the OpenTelemetry Collector.

## Status

This plugin is still being developed and is **not** ready to be used in a production grade environment.

## Configuration

```yaml
receivers:
  statsd:
    endpoint: "localhost:8125" # default
```

### endpoint

The `"<host>:<port>"` to listen on. By default listen on `"localhost:8125"`.

## Aggregation

Currently the `statsdreceiver` is not providing any aggregation. There are ideas such as the [Metrics Transform Processor Proposal](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/332) that intend to enable control over Metric aggregation in a processor.

An alternative will be to implement some simple aggregation in this receiver.

## Metrics

General format is:

`<name>:<value>|<type>|@<sample-rate>|#<tag1-key>:<tag1-value>,<tag2-k/v>`

### Counter

`<name>:<value>|c|@<sample-rate>|#<tag1-key>:<tag1-value>`

<!-- ### Gauge

`<name>:<value>|g|@<sample-rate>|#<tag1-key>:<tag1-value>`

### Timer/Histogram

`<name>:<value>|<ms/h>|@<sample-rate>|#<tag1-key>:<tag1-value>` -->

## Testing

### Full sample collector config

```yaml
receivers:
  statsd:
    endpoint: "localhost:8125" # default

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

`echo "test.metric:1|c" | nc -w 1 -u localhost 8125`
