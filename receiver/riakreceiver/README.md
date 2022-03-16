# Riak Receiver

Riak metrics will be collected through the REST API using the [/stats](https://docs.riak.com/riak/kv/2.2.3/developing/api/http/status.1.html) endpoint.

This Riak receiver will collect metrics for [release notes](https://github.com/basho/riak/releases) 3.x

Supported pipeline types: `metrics`

## Prerequisites

- None

## Configuration
The following configuration settings are required:

- `username`
- `password`

The following configuration settings are optional:

- `endpoint` (default: `http://localhost:8098`): The URL of the node to be monitored.
- `collection_interval` (default = `60s`): This receiver collects metrics on an interval. Valid time units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`.
- `tls` (defaults defined [here](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/configtls/README.md)): TLS control. By default insecure settings are rejected and certificate verification is on.

### Example Configuration

```yaml
receivers:
  riak:
    endpoint: http://localhost:8098
    username: otelu
    password: $RIAK_PASSWORD
    collection_interval: 60s
```

## Metrics

Details about the metrics produced by this receiver can be found in [metadata.yaml](./metadata.yaml)

