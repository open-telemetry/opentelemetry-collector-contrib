## `stanza_input` operator

The `stanza_input` operator acts as a source for Stanza's internal logs. It copies the logs rather than consumes them, so Stanza will still write to its log file or to stdout, depending on how it is configured. 

Care should be taken when doing any additional processing of logs coming from the `stanza_input` operator because errors from downstream processing will be passed back through the `stanza_input` operator, which can cause an infinite error loop. 

### Configuration Fields

| Field             | Default          | Description |
| ---               | ---              | ---         |
| `id`              | `stanza_input`   | A unique identifier for the operator. |
| `output`          | Next in pipeline | The connected operator(s) that will receive all outbound entries. |
| `buffer_size`     | 100              | The number of entries to buffer before dropping entries because we aren't processing fast enough. |


### Example Configurations

#### Simple Stanza input

Configuration:
```yaml
- type: stanza_input
```

Sample entry output:
```yaml
{
  "timestamp": "2020-11-06T13:55:11.314283-05:00",
  "severity": 60,
  "body": {
    "action": "send",
    "entry": {
      "timestamp": "2020-11-06T13:55:11.314057-05:00",
      "severity": 0,
      "body": "{\"key\":\"value\""
    },
    "error": "ReadMapCB: expect }, but found \u0000, error found in #10 byte of ...|y\":\"value\"|..., bigger context ...|{\"key\":\"value\"|...",
    "message": "Failed to process entry"
  }
}
```



