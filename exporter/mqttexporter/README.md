# MQTT Exporter

This exporter sends telemetry data to an MQTT broker using the MQTT protocol.

## Configuration

The following configuration options are required:

- `connection.endpoint` (no default): The MQTT broker endpoint (e.g., "tcp://localhost:1883")
- `connection.auth.plain.username` (no default): Username for authentication

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
- `connection.auth.plain.password` (optional): Password for authentication (password-less users are supported)
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
          password: "mypass"  # Optional - password-less users are supported
      client_id: "otel-collector"
      connection_timeout: 10s
      keep_alive: 30s
      publish_confirmation_timeout: 5s
      tls:
        ca_file: "/path/to/ca.crt"
        cert_file: "/path/to/cert.crt"
        key_file: "/path/to/key.key"
        insecure: false
    topic:
      topic: "telemetry/data/%{resource.attributes.host.name:unknown}"
    qos: 1
    retain: false
    encoding_extension: "otlp_encoding/otl"
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s
```

## Topic Templating

The MQTT exporter supports dynamic topic templating using resource attributes. Use the `%{resource.attributes.attribute_name}` pattern to substitute values from the telemetry data.

**Basic Examples:**
- `%{resource.attributes.host.name}` - Uses the host name from resource attributes
- `%{resource.attributes.service.name}` - Uses the service name from resource attributes
- `%{resource.attributes.environment}` - Uses the environment from resource attributes

**Default Values:**
You can provide default values for attributes that might not be present in the telemetry data by using a colon separator:
- `%{resource.attributes.host.name:unknown}` - Uses the host name if present, otherwise uses "unknown"
- `%{resource.attributes.environment:production}` - Uses the environment if present, otherwise uses "production"
- `%{resource.attributes.service.name:default-service}` - Uses the service name if present, otherwise uses "default-service"

**Advanced Examples:**
- `telemetry/data/%{resource.attributes.host.name:unknown}/%{resource.attributes.service.name:default-service}` - Creates a topic with both host name and service name, using defaults if not found
- `metrics/%{resource.attributes.environment:production}/%{resource.attributes.host.name}` - Uses environment with default, host name without default
## Supported Signal Types

- Traces
- Metrics  
- Logs

## Stability

This exporter is in **Alpha** stability for all signal types. 
