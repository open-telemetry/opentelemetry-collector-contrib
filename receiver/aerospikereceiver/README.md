# Aerospike Receiver

| Status                   |           |
| ------------------------ | --------- |
| Stability                | [alpha]   |
| Supported pipeline types | metrics   |
| Distributions            | [contrib] |

The Aerospike receiver is designed to collect performance metrics from one or
more Aerospike nodes. It uses the
[official Go client](https://github.com/aerospike/aerospike-client-go/tree/v5/)
to connect and collect.

Aerospike versions 4.9, 5.x, and 6.x are supported.


## Configuration

Configuration parameters:

- `endpoint` (default localhost:3000)
- `collect_cluster_metrics` (default false): Whether discovered peer nodes should be collected
- `collection_interval` (default = 60s): This receiver collects metrics on an interval. Valid time units are ns, us (or µs), ms, s, m, h.
- `username` (Enterprise Edition only.)
- `password` (Enterprise Edition only.)

### Example Configuration

```yaml
receivers:
    aerospike:
        endpoint: localhost:9000
        collect_cluster_metrics: false
        collection_interval: 30s
```

## Metrics

Details about the metrics produced by this receiver can be found in [metadata.yaml](./metadata.yaml)

[alpha]: https://github.com/open-telemetry/opentelemetry-collector#alpha
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
