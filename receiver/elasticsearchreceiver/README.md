# Elasticsearch Receiver

This receiver queries the Elasticsearch [node stats](https://www.elastic.co/guide/en/elasticsearch/reference/current/cluster-nodes-stats.html) and [cluster health](https://www.elastic.co/guide/en/elasticsearch/reference/current/cluster-health.html) endpoints.

Supported pipeline types: `metrics`

> :construction: This receiver is in **BETA**. Configuration fields and metric data model are subject to change.

## Prerequisites

This receiver supports Elasticsearch versions 7.9+

If Elasticsearch security features are enabled, you must have either the `monitor` or `manage` cluster privilege.
See the [Elasticsearch docs](https://www.elastic.co/guide/en/elasticsearch/reference/current/authorization.html) for more information on authorization and [Security privileges](https://www.elastic.co/guide/en/elasticsearch/reference/current/security-privileges.html).

## Configuration

The following settings are optional:
- `endpoint` (default = `http://localhost:9200`): The base URL of the Elasticsearch API for the cluster to monitor.
- `username` (no default): Specifies the username used to authenticate with Elasticsearch using basic auth. Must be specified if password is specified.
- `password` (no default): Specifies the password used to authenticate with Elasticsearch using basic auth. Must be specified if username is specified.
- `collection_interval` (default = `10s`): This receiver collects metrics on an interval. This value must be a string readable by Golang's [time.ParseDuration](https://pkg.go.dev/time#ParseDuration). On larger clusters, the interval may need to be lengthened, as querying Elasticsearch for metrics will take longer on clusters with more nodes.

### Example Configuration

```yaml
receivers:
  elasticsearch:
    endpoint: http://localhost:9200
    username: otel
    password: password
    collection_interval: 10s
```

The full list of settings exposed for this receiver are documented [here](./config.go) with detailed sample configurations [here](./testdata/config.yaml).

## Metrics

Details about the metrics produced by this receiver can be found in [metadata.yaml](./metadata.yaml)