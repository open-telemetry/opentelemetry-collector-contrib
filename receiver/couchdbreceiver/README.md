# CouchDB Receiver

This receiver fetches stats from a couchdb server using the `/_node/{node-name}/_stats/couchdb` [endpoint](https://docs.couchdb.org/en/latest/api/server/common.html#node-node-name-stats).

Supported pipeline types: `metrics`

> :construction: This receiver is in **BETA**. Configuration fields and metric data model are subject to change.

## Prerequisites

This receiver supports Couchdb versions `2.3+` and `3.1+`.

## Configuration

The following settings are required:
- `username`
- `password`

The following settings are optional:
- `endpoint` (default: `http://localhost:5984`): The URL of the couchdb endpoint

- `all_nodes` (default: false): When `all_nodes` is false, this receiver collects a single node stats with {node-name} replaced with _local. When `all_nodes` is true, then this receiver fetches all known connected nodes using the `/_membership` [endpoint](docs.couchdb.org/en/stable/cluster/nodes.html#node-management) to then collect stats for each {node-name} found.

- `collection_interval` (default = `10s`): This receiver collects metrics on an interval. This value must be a string readable by Golang's [time.ParseDuration](https://pkg.go.dev/time#ParseDuration). Valid time units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`.

### Example Configuration

```yaml
receivers:
  couchdb:
    endpoint: http://localhost:5984
    username: otelu
    password: $COUCHDB_PASSWORD
    all_nodes: false
    collection_interval: 10s
```

The full list of settings exposed for this receiver are documented [here](./config.go) with detailed sample configurations [here](./testdata/config.yaml). TLS config is documented further under the [opentelemetry collector's configtls package](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/configtls/README.md).

## Metrics

Details about the metrics produced by this receiver can be found in [metadata.yaml](./metadata.yaml)
