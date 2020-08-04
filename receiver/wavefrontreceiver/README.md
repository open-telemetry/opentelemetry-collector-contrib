# Wavefront Receiver

### Overview

The Wavefront receiver accepts metrics depends on [carbonreceiver proto and transport](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/master/receiver/carbonreceiver), It's very similar to Carbon: it is TCP based in which each received text line represents a single metric data point. They differ on the format of their textual representation. The Wavefront receiver leverages the Carbon receiver code by implementing a dedicated parser for its format.

The receiver receives the string with Wavefront metric data, and transforms it to the collector metric format. See [https://docs.wavefront.com/wavefront_data_format.html#metrics-data-format-syntax.](https://docs.wavefront.com/wavefront_data_format.html#metrics-data-format-syntax) Each line received represents a Wavefront metric in the following format:
```<metricName> <metricValue> [<timestamp>] source=<source> [pointTags]```

### Configuration

Here's an example config.
```yaml
receivers:
  wavefront:
  wavefront/allsettings:
    endpoint: localhost:8080
    tcp_idle_timeout: 5s
    extract_collectd_tags: true
```

### Config

#### tcp_idle_timeout

The max duration that a tcp connection will idle wait for new data.

default: `30s`

#### endpoint

The endpoint specifies the network interface and port which will receive Wavefront data.

default: `localhost:2003`

#### extract_collectd_tags

The extract_collectd_tags instructs the Wavefront receiver to attempt to extract tags in the CollectD format from the metric name.

default: `false`