## `otlp_output` operator

The `otlp_output` operator will send entries to an OpenTelemetry collector's [OTLP Receiver](https://github.com/open-telemetry/opentelemetry-collector/tree/master/receiver/otlpreceiver)

### Configuration Fields

| Field           | Default                           | Description                                                                                                                                                                |
| ---             | ---                               | ---                                                                                                                                                                        |
| `id`            | `otlp_output`                     | A unique identifier for the operator                                                                                                                                       |
| `endpoint`      | `https://localhost:55681/v1/logs` | The URI endpoint of the OpenTelemetry receiver. `http://` or `https://` will be prepended according to the value of `insecure`. `/v1/logs` will be appended if not present |
| `insecure`      | `false`                           | Whether or not to use TLS when sending logs to the OTLP receiver                                                                                                           |
| `buffer`        |                                   | A [buffer](/docs/types/buffer.md) block indicating how to buffer entries before flushing                                                                                   |
| `flusher`       |                                   | A [flusher](/docs/types/flusher.md) block configuring flushing behavior                                                                                                    |

Additional advanced configuration is available. See OpenTelemetry's [HTTPClientSettings](https://github.com/open-telemetry/opentelemetry-collector/blob/7dd853ab95834619169360fa2abbb981af42f061/config/confighttp/confighttp.go#L29) for more details.

### Example Configurations

#### Simple configuration

Configuration:
```yaml
- type: otlp_output
  endpoint: localhost:55681
  insecure: true
```
