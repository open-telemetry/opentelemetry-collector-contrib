## Windows Log Event Receiver

| Status                   |         |
| ------------------------ |---------|
| Stability                | [alpha] |
| Supported pipeline types | logs    |
| Distributions            | none    |

Tails and parses logs from windows event log API using the [opentelemetry-log-collection](https://github.com/open-telemetry/opentelemetry-log-collection) library.

### Configuration Fields

| Field           | Default                  | Description                                                                                                                    |
| ---             | ---                      | ---                                                                                                                            |
| `channel`       | required                 | The windows event log channel to monitor                                                                                       |
| `max_reads`     | 100                      | The maximum number of records read into memory, before beginning a new batch                                                   |
| `start_at`      | `end`                    | On first startup, where to start reading logs from the API. Options are `beginning` or `end`                                   |
| `poll_interval` | 1s                       | The interval at which the channel is checked for new log entries. This check begins again after all new bodies have been read. |
| `attributes`    | {}                       | A map of `key: value` pairs to add to the entry's attributes. |
| `resource`      | {}                       | A map of `key: value` pairs to add to the entry's resource. |
| `operators`            | []               | An array of [operators](https://github.com/open-telemetry/opentelemetry-log-collection/blob/main/docs/operators/README.md#what-operators-are-available). See below for more details |
| `converter`            | <pre lang="jsonp">{<br>  max_flush_count: 100,<br>  flush_interval: 100ms,<br>  worker_count: max(1,runtime.NumCPU()/4)<br>}</pre> | A map of `key: value` pairs to configure the [`entry.Entry`][entry_link] to [`pdata.LogRecord`][pdata_logrecord_link] converter, more info can be found [here][converter_link] |

### Operators

Each operator performs a simple responsibility, such as parsing a timestamp or JSON. Chain together operators to process logs into a desired format.

- Every operator has a `type`.
- Every operator can be given a unique `id`. If you use the same type of operator more than once in a pipeline, you must specify an `id`. Otherwise, the `id` defaults to the value of `type`.
- Operators will output to the next operator in the pipeline. The last operator in the pipeline will emit from the receiver. Optionally, the `output` parameter can be used to specify the `id` of another operator to which logs will be passed directly.
- Only parsers and general purpose operators should be used.

## Additional Terminology and Features

- An [entry](https://github.com/open-telemetry/opentelemetry-log-collection/blob/main/docs/types/entry.md) is the base representation of log data as it moves through a pipeline. All operators either create, modify, or consume entries.
- A [field](https://github.com/open-telemetry/opentelemetry-log-collection/blob/main/docs/types/field.md) is used to reference values in an entry.
- A common [expression](https://github.com/open-telemetry/opentelemetry-log-collection/blob/main/docs/types/expression.md) syntax is used in several operators. For example, expressions can be used to [filter](https://github.com/open-telemetry/opentelemetry-log-collection/blob/main/docs/operators/filter.md) or [route](https://github.com/open-telemetry/opentelemetry-log-collection/blob/main/docs/operators/router.md) entries.
- [timestamp](https://github.com/open-telemetry/opentelemetry-log-collection/blob/main/docs/types/timestamp.md) parsing is available as a block within all parser operators, and also as a standalone operator. Many common timestamp layouts are supported.
- [severity](https://github.com/open-telemetry/opentelemetry-log-collection/blob/main/docs/types/severity.md) parsing is available as a block within all parser operators, and also as a standalone operator. Stanza uses a flexible severity representation which is automatically interpreted by the stanza receiver.

### Example Configurations

#### Simple

Configuration:
```yaml
receivers:
    windowseventlog:
        channel: application
```

Output entry sample:
```json
{
    "channel": "Application",
    "computer": "computer name",
    "event_id":
    {
        "id": 10,
        "qualifiers": 0
    },
    "keywords": "[Classic]",
    "level": "Information",
    "message": "Test log",
    "opcode": "Info",
    "provider":
    {
        "event_source": "",
        "guid": "",
        "name": "otel"
    },
    "record_id": 12345,
    "system_time": "2022-04-15T15:28:08.898974100Z",
    "task": ""
}
```
[alpha]:https://github.com/open-telemetry/opentelemetry-collector#alpha
