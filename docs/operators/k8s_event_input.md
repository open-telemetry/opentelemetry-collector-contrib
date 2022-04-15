## `k8s_event_input` operator

The `k8s_event_input` operator generates logs from Kubernetes events. It does this by connecting to the
Kubernetes API, and currently requires that Stanza is running inside a Kubernetes cluster.

### Configuration Fields

| Field                 | Default           | Description |
| ---                   | ---               | ---         |
| `id`                  | `k8s_event_input` | A unique identifier for the operator. |
| `output`              | Next in pipeline  | The connected operator(s) that will receive all outbound entries. |
| `namespaces`          | All namespaces    | An array of namespaces to collect events from.. |
| `discover_namespaces` | `true`            | If true, the operator will regularly poll for new namespaces to include. |
| `discovery_interval ` | `1m`              | The interval at which the operator searches for new namespaces to follow. |
| `attributes`          | {}                | A map of `key: value` pairs to add to the entry's attributes. |
| `resource`            | {}                | A map of `key: value` pairs to add to the entry's resource. |

### Example Configurations

#### Mock a file input

Configuration:
```yaml
- type: k8s_event_input
```

Output events:
```json
{
  "timestamp": "2020-08-13T17:41:44.581552468Z",
  "severity": 0,
  "attributes": {
    "event_type": "ADDED"
  },
  "body": {
    "count": 1,
    "eventTime": null,
    "firstTimestamp": "2020-08-13T16:43:57Z",
    "involvedObject": {
      "apiVersion": "v1",
      "fieldPath": "spec.containers{stanza}",
      "kind": "Pod",
      "name": "stanza-g6rzd",
      "namespace": "default",
      "resourceVersion": "18292818",
      "uid": "47d965e6-4bb3-4c58-a089-1a8b16bf21b0"
    },
    "lastTimestamp": "2020-08-13T16:43:57Z",
    "message": "Pulling image \"observiq/stanza:dev\"",
    "metadata": {
      "creationTimestamp": "2020-08-13T16:43:57Z",
      "name": "stanza-g6rzd.162ae19292cebe25",
      "namespace": "default",
      "resourceVersion": "29923",
      "selfLink": "/api/v1/namespaces/default/events/stanza-g6rzd.162ae19292cebe25",
      "uid": "d210b74b-5c58-473f-ac51-3e21f6f8e2d1"
    },
    "reason": "Pulling",
    "reportingComponent": "",
    "reportingInstance": "",
    "source": {
      "component": "kubelet",
      "host": "kube-master-1"
    },
    "type": "Normal"
  }
}
```
