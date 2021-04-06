## `k8s_metadata_decorator` operator

The `k8s_metadata_decorator` operator adds attributes to the entry using data from the Kubernetes metadata API.

### Configuration Fields

| Field             | Default                  | Description                                                                                                                                                                                                                              |
| ---               | ---                      | ---                                                                                                                                                                                                                                      |
| `id`              | `k8s_metadata_decorator` | A unique identifier for the operator                                                                                                                                                                                                     |
| `output`          | Next in pipeline         | The connected operator(s) that will receive all outbound entries                                                                                                                                                                         |
| `namespace_field` | `namespace`              | A [field](/docs/types/field.md) that contains the k8s namespace associated with the log entry                                                                                                                                            |
| `pod_name_field`  | `pod_name`               | A [field](/docs/types/field.md) that contains the k8s pod name associated with the log entry                                                                                                                                             |
| `cache_ttl`       | 10m                      | A [duration](/docs/types/duration.md) indicating the time it takes for a cached entry to expire                                                                                                                                          |
| `timeout`         | 10s                      | A [duration](/docs/types/duration.md) indicating how long to wait for the API to respond before timing out                                                                                                                               |
| `allow_proxy`     | false                    | Controls whether or not the agent will take into account [proxy](https://github.com/open-telemetry/opentelemetry-log-collection/blob/master/docs/proxy.md) configuration when communicating with the k8s metadata api |
| `if`              |                          | An [expression](/docs/types/expression.md) that, when set, will be evaluated to determine whether this operator should be used for the given entry. This allows you to do easy conditional parsing without branching logic with routers. |

### Example Configurations


#### Add attributes to a log entry

Configuration:
```yaml
- type: k8s_metadata_decorator
```

<table>
<tr><td> Input body </td> <td> Output body </td></tr>
<tr>
<td>

```json
{
  "timestamp": "",
  "body": {
    "namespace": "my-namespace",
    "pod_name": "samplepod-6cdcf6bf9d-f4f9n"
  }
}
```

</td>
<td>

```json
{
  "timestamp": "",
  "attributes": {
    "k8s_ns_annotation/addonmanager.kubernetes.io/mode": "Reconcile",
    "k8s_ns_annotation/control-plane": "true",
    "k8s_ns_annotation/kubernetes.io/cluster-service": "true",
    "k8s_ns_label/addonmanager.kubernetes.io/mode": "Reconcile",
    "k8s_ns_label/control-plane": "true",
    "k8s_ns_label/kubernetes.io/cluster-service": "true",
    "k8s_pod_annotation/k8s-app": "dashboard-metrics-scraper",
    "k8s_pod_annotation/pod-template-hash": "5f44bbb8b5",
    "k8s_pod_label/k8s-app": "dashboard-metrics-scraper",
    "k8s_pod_label/pod-template-hash": "5f44bbb8b5"
  },
  "body": {
    "namespace": "my-namespace",
    "pod_name": "samplepod-6cdcf6bf9d-f4f9n"
  }
}
```

</td>
</tr>
</table>
