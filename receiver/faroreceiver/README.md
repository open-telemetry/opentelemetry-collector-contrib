# Faro Receiver

This receiver can receive telemetry data from the [Grafana Faro Web SDK](https://github.com/grafana/faro-web-sdk).
The telemetry data must be in JSON format and adhere to the [Faro OpenAPI schema](https://github.com/grafana/faro/blob/main/spec/gen/faro.gen.yaml).

## Receiver Configuration

Faro follows the [confighttp] configuration, some examples are shown below


### Example Configuration

```yaml
receivers:
  faro:
    endpoint: 'localhost:8081'
```

### Advanced Configuration

```yaml
receivers:
  faro:
    endpoint: 'localhost:8081'
    cors:
      allowed_origins: "*"
```

[confighttp]: https://github.com/open-telemetry/opentelemetry-collector/tree/main/config/confighttp#server-configuration