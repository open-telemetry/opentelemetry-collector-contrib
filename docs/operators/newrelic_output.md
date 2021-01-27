## `newrelic_output` operator

The `newrelic_output` operator will send entries to the New Relic Logs platform

### Configuration Fields

| Field           | Default                               | Description                                                                                                               |
| ---             | ---                                   | ---                                                                                                                       |
| `id`            | `newrelic_output`                     | A unique identifier for the operator                                                                                      |
| `api_key`       |                                       | An API key for your account                                                                                               |
| `license_key`   |                                       | A license key for your account                                                                                            |
| `base_uri`      | `https://log-api.newrelic.com/log/v1` | The URI endpoint to send logs to                                                                                          |
| `message_field` | `$record`                             | A [field](/docs/types/field.md) that points to the field that will be promoted to the top-level message in New Relic Logs |
| `timeout`       | 10s                                   | A [duration](/docs/types/duration.md) indicating how long to wait for the API to respond before timing out                |
| `buffer`        |                                       | A [buffer](/docs/types/buffer.md) block indicating how to buffer entries before flushing                                  |
| `flusher`       |                                       | A [flusher](/docs/types/flusher.md) block configuring flushing behavior                                                   |

Only one of `api_key` or `license_key` are required. You can find your logs in the New Relic One UI by filtering to `plugin.type:"stanza"`.

### Example Configurations

#### Simple configuration

Configuration:
```yaml
- type: newrelic_output
  api_key: <my_api_key>
```
