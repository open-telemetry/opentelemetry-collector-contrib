# Elasticsearch Receiver

| Status                   |           |
| ------------------------ |-----------|
| Stability                | [beta]    |
| Supported pipeline types | metrics   |
| Distributions            | [contrib] |

This receiver queries the Elasticsearch [node stats](https://www.elastic.co/guide/en/elasticsearch/reference/current/cluster-nodes-stats.html) and [cluster health](https://www.elastic.co/guide/en/elasticsearch/reference/current/cluster-health.html) endpoints in order to scrape metrics from a running elasticsearch cluster.

## Prerequisites

This receiver supports Elasticsearch versions 7.9+

If Elasticsearch security features are enabled, you must have either the `monitor` or `manage` cluster privilege.
See the [Elasticsearch docs](https://www.elastic.co/guide/en/elasticsearch/reference/current/authorization.html) for more information on authorization and [Security privileges](https://www.elastic.co/guide/en/elasticsearch/reference/current/security-privileges.html).

## Configuration

The following settings are optional:
- `metrics` (default: see `DefaultMetricsSettings` [here](./internal/metadata/generated_metrics.go): Allows enabling and disabling specific metrics from being collected in this receiver.
- `nodes` (default: `["_all"]`): Allows specifying node filters that define which nodes are scraped for node-level metrics. See [the Elasticsearch documentation](https://www.elastic.co/guide/en/elasticsearch/reference/7.9/cluster.html#cluster-nodes) for allowed filters. If this option is left explicitly empty, then no node-level metrics will be scraped.
- `skip_cluster_metrics` (default: `false`): If true, cluster-level metrics will not be scraped.
- `endpoint` (default = `http://localhost:9200`): The base URL of the Elasticsearch API for the cluster to monitor.
- `username` (no default): Specifies the username used to authenticate with Elasticsearch using basic auth. Must be specified if password is specified.
- `password` (no default): Specifies the password used to authenticate with Elasticsearch using basic auth. Must be specified if username is specified.
- `collection_interval` (default = `10s`): This receiver collects metrics on an interval. This value must be a string readable by Golang's [time.ParseDuration](https://pkg.go.dev/time#ParseDuration). On larger clusters, the interval may need to be lengthened, as querying Elasticsearch for metrics will take longer on clusters with more nodes.

### Example Configuration

```yaml
receivers:
  elasticsearch:
    metrics:
      elasticsearch.node.fs.disk.available:
        enabled: false
    nodes: ["_local"]
    skip_cluster_metrics: true
    endpoint: http://localhost:9200
    username: otel
    password: password
    collection_interval: 10s
```

The full list of settings exposed for this receiver are documented [here](./config.go) with detailed sample configurations [here](./testdata/config.yaml).

## Metrics

Details about the metrics produced by this receiver can be found in [metadata.yaml](./metadata.yaml)

### Feature gate configurations

#### Transition from metrics with "direction" attribute

There is a proposal to change some elasticsearch metrics from being reported with a `direction` attribute to being
reported with the direction included in the metric name.

- `elasticsearch.node.cluster.io` will become:
  - `elasticsearch.node.cluster.io.received`
  - `elasticsearch.node.cluster.io.sent`

The following feature gates control the transition process:

- **receiver.elasticsearchreceiver.emitMetricsWithoutDirectionAttribute**: controls if the new metrics without `direction` attribute are emitted by the receiver.
- **receiver.elasticsearchreceiver.emitMetricsWithDirectionAttribute**: controls if the deprecated metrics with `direction` attribute are emitted by the receiver.

##### Transition schedule:

The final decision on the transition is not finalized yet. The transition is on hold until
https://github.com/open-telemetry/opentelemetry-specification/issues/2726 is resolved.

[beta]:https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
