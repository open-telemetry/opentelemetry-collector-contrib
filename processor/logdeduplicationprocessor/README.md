# Log DeDuplication Processor
This processor is used to deduplicate logs by detecting identical logs over a range of time and emitting a single log with the count of logs that were deduplicated.

## Supported pipelines
- Logs

## How It Works
1. The user configures the log deduplication processor in the desired logs pipeline.
2. All logs sent to the processor and aggregated over the configured `interval`. Logs are considered identical if they have the same body, resource attributes, severity, and log attributes.
3. After the interval, the processor emits a single log with the count of logs that were deduplicated. The emitted log will have the same body, resource attributes, severity, and log attributes as the original log. The emitted log will also have the following new attributes:

    - `log_count`: The count of logs that were deduplicated over the interval. The name of the attribute is configurable via the `log_count_attribute` parameter.
    - `first_observed_timestamp`: The timestamp of the first log that was observed during the aggregation interval.
    - `last_observed_timestamp`: The timestamp of the last log that was observed during the aggregation interval.

**Note**: The `ObservedTimestamp` and `Timestamp` of the emitted log will be the time that the aggregated log was emitted and will not be the same as the `ObservedTimestamp` and `Timestamp` of the original logs.

## Configuration
| Field        | Type     | Default | Description |
| ---          | ---      | ---     | ---         |
| interval     | duration | `10s`    | The interval at which logs are aggregated. The counter will reset after each interval. |
| log_count_attribute     | string | `log_count`    | The name of the count attribute of deduplicated logs that will be added to the emitted aggregated log. |
| timezone     | string | `UTC`    | The timezone of the `first_observed_timestamp` and `last_observed_timestamp` timestamps on the emitted aggregated log. Valid values listed [here](../../docs/timezone.md) |
| exclude_fields     | []string | `[]`    | Fields to exclude from duplication matching. Fields can be excluded from the log `body` or `attributes`. These fields will not be present in the emitted aggregated log. Nested fields must be `.` delimited. If a field contains a `.` it can be escaped by using a `\` see [example config](#example-config-with-excluded-fields).<br><br>**Note**: The entire `body` cannot be excluded. If the body is a map then fields within it can be excluded. |


### Example Config
The following config is an example configuration for the log deduplication processor. It is configured with an aggregation interval of `60 seconds`, a timezone of `America/Los_Angeles`, and a log count attribute of `dedup_count`. It has no fields being excluded.
```yaml
receivers:
    filelog:
        include: [./example/*.log]
processors:
    logdedup:
        interval: 60s
        log_count_attribute: dedup_count
        timezone: 'America/Los_Angeles'
exporters:
    googlecloud:

service:
    pipelines:
        logs:
            receivers: [filelog]
            processors: [logdedup]
            exporters: [googlecloud]
```

### Example Config with Excluded Fields
The following config is an example configuration that excludes the following fields from being considered when searching for duplicate logs:

- `timestamp` field from the body
- `host.name` field from attributes
- `ip` nested attribute inside a map attribute named `src`

```yaml
receivers:
    filelog:
        include: [./example/*.log]
processors:
    logdedup:
        exclude_fields:
          - body.timestamp
          - attributes.host\.name
          - attributes.src.ip
exporters:
    googlecloud:

service:
    pipelines:
        logs:
            receivers: [filelog]
            processors: [logdedup]
            exporters: [googlecloud]
```
