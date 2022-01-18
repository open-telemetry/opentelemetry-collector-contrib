# Couchbase Receiver

This receiver fetches stats from a couchbase cluster using the following endpoints:
- `/pools/default` [endpoint](https://docs.couchbase.com/server/6.5/rest-api/rest-cluster-details.html)
- `/pools/default/buckets/{bucket_name}/stats` [endpoint](https://docs.couchbase.com/server/6.5/rest-api/rest-bucket-stats.html).

Supported pipeline types: `metrics`

> :construction: This receiver is in **BETA**. Configuration fields and metric data model are subject to change.
## Prerequisites

This receiver supports Couchbase versions `6.5`, `6.6`, and `7.0`.

## Configuration

The following settings are required:
- `endpoint` (default: `http://localhost:8091`): The URL of any node within the cluster to be monitored.
- `username`
- `password`

The following settings are optional:

- `collection_interval` (default = `10s`): This receiver collects metrics on an interval. Valid time units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`.
- `tls` (defaults defined [here](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/configtls/README.md)): TLS control. By default insecure settings are rejected and certificate verification is on.

### Example Configuration

```yaml
receivers:
  couchbase:
    endpoint: http://localhost:8901
    username: otelu
    password: $COUCHBASE_PASSWORD
    collection_interval: 10s
```

The full list of settings exposed for this receiver are documented [here](./config.go) with detailed sample configurations [here](./testdata/config.yaml). TLS config is documented further under the [opentelemetry collector's configtls package](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/configtls/README.md).

## Metrics

Details about the metrics produced by this receiver can be found in [metadata.yaml](./metadata.yaml)