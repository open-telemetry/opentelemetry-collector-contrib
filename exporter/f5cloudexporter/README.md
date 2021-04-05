# F5 Cloud Exporter

Exports data via gRPC to [F5 Cloud](https://portal.cloudservices.f5.com/) using [OTLP](
https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/protocol/otlp.md)
format. By default, this exporter requires TLS and offers queued retry capabilities.

Supported pipeline types: metrics, traces, logs

> :construction: This exporter is in beta and configuration fields are subject to change.

## Getting Started

The following settings are required:

- `endpoint` (no default): host:port to which the exporter is going to send OTLP trace data,
  using the gRPC protocol. The valid syntax is described
  [here](https://github.com/grpc/grpc/blob/master/doc/naming.md). See your F5 Cloud account for details.
- `source` (no default): A unique identifier that is used to distinguish where this data is coming from (e.g. dev_cluster). This is in 
  addition to the pipeline attributes and resources.
- `auth.credential_file` (no default): Path to the credential file used to authenticate this client. See your F5 
  Cloud account for details.

The following settings can be optionally configured:

- `auth.audience` (no default): Identifies the recipient that the authentication JWT is intended for. See your F5 Cloud 
  account for details.

Example:

```yaml
f5cloud:
  endpoint: <ENDPOINT_FOUND_IN_F5_CLOUD_PORTAL>
  source: prod
  auth:
    credential_file: "/etc/creds/key.json"
```

The full list of settings exposed for this exporter are documented [here](./config.go) with detailed sample
configurations [here](./testdata/config.yaml).

This exporter also offers proxy support as documented 
[here](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter#proxy-support).

## Advanced Configuration

Several helper files are leveraged to provide additional capabilities automatically:

- [gRPC settings](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/configgrpc/README.md)
- [TLS and mTLS settings](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/configtls/README.md)
- [Queuing, retry and timeout settings](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/exporterhelper/README.md)