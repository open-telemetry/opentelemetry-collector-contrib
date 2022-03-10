# Windows Performance Counters Receiver

This receiver, for Windows only, captures the configured system, application, or
custom performance counter data from the Windows registry using the [PDH
interface](https://docs.microsoft.com/en-us/windows/win32/perfctrs/using-the-pdh-functions-to-consume-counter-data).
It is based on the [Telegraf Windows Performance Counters Input
Plugin](https://github.com/influxdata/telegraf/tree/master/plugins/inputs/win_perf_counters).

- `Memory\Committed Bytes`
- `Processor\% Processor Time`, with a datapoint for each `Instance` label = (`_Total`, `1`, `2`, `3`, ... )

If one of the specified performance counters cannot be loaded on startup, a
warning will be printed, but the application will not fail fast. It is expected
that some performance counters may not exist on some systems due to different OS
configuration.

## Configuration

The collection interval and the list of performance counters to be scraped can
be configured:

```yaml
windowsperfcounters:
  collection_interval: <duration> # default = "1m"
  metric_metadata:
  - metric_name: <metric name>
    description: <description>
    unit: <unit type>
    gauge:
      value_type: <int or double>
  - metric_name: <metric name>
    description: <description>
    unit: <unit type>
    sum: 
      value_type: <int or double>
      aggregation: <cumulative or delta>
      monotonic: <true or false>
  perfcounters:
    - object: <object name>
      instances: [<instance name>]*
      counters:
        - counter_name: <counter name>
          metric_name: <metric name>
          attributes:
            <key>: <value>
```

*Note `instances` can have several special values depending on the type of
counter:

Value | Interpretation
-- | --
Not specified | This is the only valid value if the counter has no instances
`"*"` | All instances
`"_Total"` | The "total" instance
`"instance1"` | A single instance
`["instance1", "instance2", ...]` | A set of instances
`["_Total", "instance1", "instance2", ...]` | A set of instances including the "total" instance

### Scraping at different frequencies

If you would like to scrape some counters at a different frequency than others,
you can configure multiple `windowsperfcounters` receivers with different
`collection_interval` values. For example:

```yaml 
receivers:
  windowsperfcounters/memory:
    metric_metadata:
      - metric_name: bytes.committed
        description: the number of bytes committed to memory
        unit: By
        gauge:
          value_type: int
    collection_interval: 30s
    perfcounters:
      - object: Memory
        counters:
          - counter_name: Committed Bytes
            metric_name: bytes.committed

  windowsperfcounters/processor:
    collection_interval: 1m
    metric_metadata:
      - metric_name: processor.time
        description: active and idle time of the processor
        unit: "%"
        gauge:
          value_type: double
    perfcounters:
      - object: "Processor"
        instances: "*"
        counters:
          - counter_name: "% Processor Time"
            metric_name: processor.time
            attributes:
              state: active
      - object: "Processor"
        instances: [1, 2]
        counters:
          - counter_name: "% Idle Time"
            metric_name: processor.time
            attributes:
              state: idle

service:
  pipelines:
    metrics:
      receivers: [windowsperfcounters/memory, windowsperfcounters/processor]
```

### Defining metric format

To report metrics in the desired output format, build a metric the metric and reference it in the given counter with any applicable attributes.

e.g. To output the `Memory/Committed Bytes` counter as a metric with the name
`bytes.committed`:

```yaml
receivers:
  windowsperfcounters:
    metric_metadata:
    - metric_name: bytes.committed
      description: the number of bytes committed to memory
      unit: By
      gauge:
        value_type: int
    collection_interval: 30s
    perfcounters:
    - object: Memory
      counters:
        - Committed Bytes

service:
  pipelines:
    metrics:
      receivers: [windowsperfcounters]
```

## Recommended configuration for common applications

### IIS

TODO

### SQL Server

TODO

## Known Limitation
- The network interface is not available inside the container. Hence, the metrics for the object `Network Interface` aren't generated in that scenario. In the case of sub-process, it captures `Network Interface` metrics. There is a similar open issue in [Github](https://github.com/influxdata/telegraf/issues/5357) and [Docker](https://forums.docker.com/t/unable-to-collect-network-metrics-inside-windows-container-on-windows-server-2016-data-center/69480) forum.
