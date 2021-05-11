# Windows Performance Counters Receiver

This receiver, for Windows only, captures the configured system, application, or
custom performance counter data from the Windows registry using the [PDH
interface](https://docs.microsoft.com/en-us/windows/win32/perfctrs/using-the-pdh-functions-to-consume-counter-data).
It is based on the [Telegraf Windows Performance Counters Input
Plugin](https://github.com/influxdata/telegraf/tree/master/plugins/inputs/win_perf_counters).

Metrics will be generated with names and labels that match the performance
counter path, i.e.

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
  counters:
    - object: <object name>
      instances: [<instance name>]*
      counters:
        - <counter name>
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
    collection_interval: 30s
    counters:
      - object: Memory
        counters:
          - Committed Bytes

  windowsperfcounters/processor:
    collection_interval: 1m
    counters:
      - object: "Processor"
        instances: "*"
        counters:
          - "% Processor Time"
      - object: "Processor"
        instances: [1, 2]
        counters:
          - "% Idle Time"

service:
  pipelines:
    metrics:
      receivers: [windowsperfcounters/memory, windowsperfcounters/processor]
```

### Changing metric format

To report metrics in the desired output format, it's recommended you use this
receiver with the [metrics transform
processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/metricstransformprocessor).

e.g. To output the `Memory/Committed Bytes` counter as a metric with the name
`system.memory.usage`:

```yaml
receivers:
  windowsperfcounters:
    collection_interval: 30s
    counters:
    - object: Memory
      counters:
        - Committed Bytes

processors:
  metricstransformprocessor:
    transforms:
      - metric_name: "Memory/Committed Bytes"
        action: update
        new_name: system.memory.usage

service:
  pipelines:
    metrics:
      receivers: [windowsperfcounters]
      processors: [metricstransformprocessor]
```

## Recommended configuration for common applications

### IIS

TODO

### SQL Server

TODO
