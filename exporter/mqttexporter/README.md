# MQTT Exporter

This exporter sends telemetry data to an MQTT broker using the MQTT protocol.

## Configuration

The following configuration options are required:

- `connection.endpoint` (no default): The MQTT broker endpoint (e.g., "tcp://localhost:1883")
- `connection.auth.plain.username` (no default): Username for authentication
- `connection.auth.plain.password` (no default): Password for authentication

The following configuration options can also be configured:

- `topic.topic` (default: varies by signal type): The MQTT topic to publish to
  - Default for traces: "otlp/spans"
  - Default for metrics: "otlp/metrics" 
  - Default for logs: "otlp/logs"
- `qos` (default: 1): Quality of Service level (0, 1, or 2)
- `retain` (default: false): Whether to set the retain flag on published messages
- `connection.client_id` (default: varies by signal type): MQTT client ID
  - Default for traces: "otel-collector-spans"
  - Default for metrics: "otel-collector-metrics"
  - Default for logs: "otel-collector-logs"
- `connection.connection_timeout` (default: 10s): Connection timeout
- `connection.keep_alive` (default: 30s): Keep alive interval
- `connection.publish_confirmation_timeout` (default: 5s): Timeout for publish confirmation
- `connection.tls` (optional): TLS configuration for secure connections
- `encoding_extension` (optional): Encoding extension to use for serialization
- `retry_on_failure` (optional): Configuration for retry logic

## Example Configuration

```yaml
exporters:
  mqtt:
    connection:
      endpoint: "tcp://localhost:1883"
      auth:
        plain:
          username: "myuser"
          password: "mypass"
      client_id: "otel-collector"
      connection_timeout: 10s
      keep_alive: 30s
    topic:
      topic: "telemetry/data"
    qos: 1
    retain: false
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s
```

## Supported Signal Types

- Traces
- Metrics  
- Logs

## Stability

This exporter is in **Alpha** stability for all signal types. 
