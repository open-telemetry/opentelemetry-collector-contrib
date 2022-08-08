# MongoDB Atlas Receiver

| Status                   |               |
|--------------------------|---------------|
| Stability                | [beta]        |
| Supported pipeline types | metrics, logs |
| Distributions            | [contrib]     |

Receives metrics from [MongoDB Atlas](https://www.mongodb.com/cloud/atlas) 
via their [monitoring APIs](https://docs.atlas.mongodb.com/reference/api/monitoring-and-logs/),
as well as alerts via a configured [webhook](https://www.mongodb.com/docs/atlas/tutorial/third-party-service-integrations/).

## Getting Started

The MongoDB Atlas receiver takes the following parameters. `public_key` and 
`private_key` are the only two required values to receive metrics and are obtained via the 
"API Keys" tab of the MongoDB Atlas Project Access Manager. In the example
below both values are being pulled from the environment.

- `public_key` (required for metrics)
- `private_key` (required for metrics)
- `granularity` (default `PT1M` - See [MongoDB Atlas Documentation](https://docs.atlas.mongodb.com/reference/api/process-measurements/))
- `retry_on_failure`
  - `enabled` (default true)
  - `initial_interval` (default 5s)
  - `max_interval` (default 30s)
  - `max_elapsed_time` (default 5m)
- `alerts`
  - `enabled` (default false)
  - `secret` (required if enabled)
  - `endpoint` (required if enabled)
  - `tls`
    - `key_file`
    - `cert_file`

Examples:

Receive metrics:
```yaml
receivers:
  mongodbatlas:
    public_key: ${MONGODB_ATLAS_PUBLIC_KEY}
    private_key: ${MONGODB_ATLAS_PRIVATE_KEY}
```

Receive alerts:
```yaml
receivers:
  mongodbatlas:
    alerts:
      enabled: true
      secret: "some_secret"
      endpoint: "0.0.0.0:7706"
```

[beta]:https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
