
# Mongodb Receiver

This receiver fetches stats from a Mongodb instance using the [golang
mongo driver](https://github.com/mongodb/mongo-go-driver). Stats are collected
via MongoDB's `dbStats` command.

Supported pipeline types: `metrics`

> :construction: This receiver is in **BETA**. Configuration fields and metric data model are subject to change.
## Prerequisites

This receiver supports MongoDB versions 4.0+

## Configuration

The following settings are required:
- `hosts` (default: [`localhost:27017`]): list of <host>:<port> endpoints.
  - For standalone MongoDB deployments this is the hostname and port of the mongod instance
  - For replica sets specify the hostnames and ports of the mongod instances that are in the replica set configuration. Another requirement is that the `replica_set` field must also be set with the name of the replica set for autodiscovery to work.
  - For a sharded MongoDB deployment, please specify a list of the `mongos` hosts.

The following settings are optional:
- `username`: If authentication is required, the user can be provided here.
- `password`: If authentication is required, the password can be provided here.
- `collection_interval` (default = `10s`): This receiver collects metrics on an interval. This value must be a string readable by Golang's [time.ParseDuration](https://pkg.go.dev/time#ParseDuration). Valid time units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`.
- `replica_set`: If the deployment of MongoDB is a replica set then this allows users to specify the replica set name which allows for autodiscovery of other nodes in the replica set.
- `tls` (defaults defined [here](https://github.com/open-telemetry/opentelemetry-collector/blob/main/config/configtls/README.md)): TLS control. By default insecure settings are rejected and certificte verification is on.

### Example Configuration

```yaml
receivers:
  mongodb:
    endpoint: localhost:27017
    username: otel
    password: $MONGODB_PASSWORD
    collection_interval: 60s
    tls:
      insecure: true
      insecure_skip_verify: true
```

The full list of settings exposed for this receiver are documented [here](./config.go) with detailed sample configurations [here](./testdata/config.yaml).

## Metrics

Details about the metrics produced by this receiver can be found in [metadata.yaml](./metadata.yaml)
