## Windows Log Event Receiver

The `windowseventlog` receiver reads logs from the windows event log API.

### Configuration Fields

| Field           | Default                  | Description                                                                                                                    |
| ---             | ---                      | ---                                                                                                                            |
| `id`            | `windowseventlog` | A unique identifier for the operator                                                                                           |
| `output`        | Next in pipeline         | The connected operator(s) that will receive all outbound entries                                                               |
| `channel`       | required                 | The windows event log channel to monitor                                                                                       |
| `max_reads`     | 100                      | The maximum number of records read into memory, before beginning a new batch                                                   |
| `start_at`      | `end`                    | On first startup, where to start reading logs from the API. Options are `beginning` or `end`                                   |
| `poll_interval` | 1s                       | The interval at which the channel is checked for new log entries. This check begins again after all new records have been read |
| `write_to`      | $                        | The record [field](/docs/types/field.md) written to when creating a new log entry                                              |
| `labels`        | {}                       | A map of `key: value` labels to add to the entry's labels                                                                      |
| `resource`      | {}                       | A map of `key: value` labels to add to the entry's resource                                                                    |

### Example Configurations

#### Simple

Configuration:
```yaml
- type: windowseventlog
  channel: application
```

Output entry sample:
```json
{
  "body": {
		"event_id": {
			"qualifiers": 0,
			"id": 1000
		},
		"provider": {
			"name": "provider name",
			"guid": "provider guid",
			"event_source": "event source"
		},
		"system_time": "2020-04-30T12:10:17.656726789Z",
		"computer": "example computer",
		"channel": "application",
		"record_id": 1,
		"level": "Information",
		"message": "example message",
		"task": "example task",
		"opcode": "example opcode",
		"keywords": ["example keyword"]
	}
}
```