# Windows Performance Counters Receiver

#### :warning: This receiver is still under construction. It currently only supports very basic functionality, i.e. performance counters with no 'Instance'.

This receiver, for Windows only, captures the configured system, application, or
custom performance counter data from the Windows registry using the [PDH
interface](https://docs.microsoft.com/en-us/windows/win32/perfctrs/using-the-pdh-functions-to-consume-counter-data).

## Configuration

The collection interval and the list of performance counters to be scraped can
be configured:

```yaml
windowsperfcounters:
  collection_interval: <duration> # default = "1m"
  counters:
    - object: <object name>
      counters:
        - <counter name>
```

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

  windowsperfcounters/connections:
    collection_interval: 1m
    counters:
    - object: TCPv4
      counters:
        - Connections Established

service:
  pipelines:
    metrics:
      receivers: [windowsperfcounters/memory, windowsperfcounters/connections]
```

### Changing metric format

To report metrics in the desired output format, it's recommended you use this
receiver with the metrics transform processor, e.g.:

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
      # rename "Memory/Committed Bytes" -> system.memory.usage
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
