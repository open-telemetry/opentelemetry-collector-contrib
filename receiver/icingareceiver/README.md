# Icinga Receiver

This receiver reads metric event streams from the [Icinga2 API](https://icinga.com/docs/icinga-2/latest/doc/12-icinga2-api/).

Supported pipeline types: metrics

## Configuration

The following configuration options are supported:

* `host` (default = localhost:5665) HTTP service endpoint for the Icinga API receiver
* `username` (default = root) Icinga API username
* `password` (default = "") Icinga API password
* `filter` (default = "") Optional Icinga Event [advanced filter expression](https://icinga.com/docs/icinga-2/latest/doc/12-icinga2-api/#advanced-filters). You can use attributes in the [CheckResult type](https://icinga.com/docs/icinga-2/latest/doc/12-icinga2-api/#event-stream-type-checkresult).
* `histograms` (default = []) Optionally generated histogram metrics based on custom histogram steps. 

The full list of settings exposed for this receiver are documented in [config.go](config.go).

Example:
```yaml
receivers:
  icinga:
    host: localhost:5665
    username: root
    password: test
    filter: "match(\"foo_*\", event.service)"
    histograms:
      - service: my_service
        type: time
        host: .*
        values: [5, 10, 25, 100]
```

### Histograms

This config option allows you to generate additional histogram metrics for any combination of `service`, `type`
and `host`. Each property supports regular expressions. If you leave any of them empty, it will match all.

E.g. the above example would generate these Prometheus metrics:
```
icinga.histogram.service.perf.millis{le="5.000000",service_instance_id="demo",service_name="foo_bar",state="0",type="time"} 5
icinga.histogram.service.perf.millis{le="10.000000",service_instance_id="demo",service_name="foo_bar",state="0",type="time"} 8
icinga.histogram.service.perf.millis{le="25.000000",service_instance_id="demo",service_name="foo_bar",state="0",type="time"} 10
icinga.histogram.service.perf.millis{le="100.000000",service_instance_id="demo",service_name="foo_bar",state="0",type="time"} 10
icinga.histogram.service.perf.millis{le="+Inf",service_instance_id="demo",service_name="foo_bar",state="0",type="time"} 10
```

## Definitions

[Icinga](https://icinga.com/) is an infrastructure monitoring tool forked from [Nagios](https://www.nagios.org/).

Icinga2 is the currently supported version of Icinga. It provides a non-terminating 
[HTTP endpoint](https://icinga.com/docs/icinga-2/latest/doc/12-icinga2-api/#icinga2-api-event-streams) that streams all events. 
This receiver opens a connection to said endpoint and updates counter, gauge and histogram metrics associated to each event. 

If the check result is not associated to a host, the metric will start with `icinga.{type}.host`. If a service is associated
with the check result, metrics will start with `icinga.{type}.service`.

The receiver also tries to normalize performance data.

- Durations such as seconds are converted to milliseconds
- Information units such as mega bytes are converted to bytes

Gauges and histogram metrics are ending with `.perf.{unit}`. 
