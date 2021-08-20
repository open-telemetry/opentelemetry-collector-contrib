# Kubelet Stats Receiver

The Kubelet Stats Receiver pulls pod metrics from the API server on a kubelet
and sends it down the metric pipeline for further processing.

Supported pipeline types: metrics

> :construction: This receiver is in beta and configuration fields are subject to change.

## Configuration

A kubelet runs on a kubernetes node and has an API server to which this
receiver connects. To configure this receiver, you have to tell it how
to connect and authenticate to the API server and how often to collect data
and send it to the next consumer.

There are two ways to authenticate, driven by the `auth_type` field:

- `tls` tells the receiver to use TLS for auth and requires that the fields
`ca_file`, `key_file`, and `cert_file` also be set.
- `ServiceAccount` tells this receiver to use the default service account token
to authenticate to the kubelet API.

### TLS Example

```yaml
receivers:
  kubeletstats:
    collection_interval: 20s
    auth_type: "tls"
    ca_file: "/path/to/ca.crt"
    key_file: "/path/to/apiserver.key"
    cert_file: "/path/to/apiserver.crt"
    endpoint: "192.168.64.1:10250"
    insecure_skip_verify: true
exporters:
  file:
    path: "fileexporter.txt"
service:
  pipelines:
    metrics:
      receivers: [kubeletstats]
      exporters: [file]
```

### ServiceAccount Example

Although it's possible to use kubernetes' hostNetwork feature to talk to the
kubelet api from a pod, the preferred approach is to use the downward API.

Make sure the pod spec sets the node name as follows:

```yaml
env:
  - name: K8S_NODE_NAME
    valueFrom:
      fieldRef:
        fieldPath: spec.nodeName
```

Then the otel config can reference the `K8S_NODE_NAME` environment variable:

```yaml
receivers:
  kubeletstats:
    collection_interval: 20s
    auth_type: "serviceAccount"
    endpoint: "${K8S_NODE_NAME}:10250"
    insecure_skip_verify: true
exporters:
  file:
    path: "fileexporter.txt"
service:
  pipelines:
    metrics:
      receivers: [kubeletstats]
      exporters: [file]
```

Note: a missing or empty `endpoint` will cause the hostname on which the
collector is running to be used as the endpoint. If the hostNetwork flag is
set, and the collector is running in a pod, this hostname will resolve to the
node's network namespace.

### Extra metadata labels

By default, all produced metrics get resource labels based on what kubelet /stats/summary endpoint provides.
For some use cases it might be not enough. So it's possible to leverage other endpoints to fetch
additional metadata entities and set them as extra labels on metric resource. Currently supported metadata
include the following:

- `container.id` - to augment metrics with Container ID label obtained from container statuses exposed via `/pods`.
- `k8s.volume.type` - to collect volume type from the Pod spec exposed via `/pods` and have it as a label on volume metrics.
If there's more information available from the endpoint than just volume type, those are sycned as well depending on
the available fields and the type of volume. For example, `aws.volume.id` would be synced from `awsElasticBlockStore`
and `gcp.pd.name` is synced for `gcePersistentDisk`.

If you want to have `container.id` label added to your metrics, use `extra_metadata_labels` field to enable
it, for example:

```yaml
receivers:
  kubeletstats:
    collection_interval: 10s
    auth_type: "serviceAccount"
    endpoint: "${K8S_NODE_NAME}:10250"
    insecure_skip_verify: true
    extra_metadata_labels:
      - container.id
```

If `extra_metadata_labels` is not set, no additional API calls is done to fetch extra metadata.

#### Collecting Additional Volume Metadata

When dealing with Persistent Volume Claims, it is possible to optionally sync metdadata from the underlying
storage resource rather than just the volume claim. This is achieved by talking to the Kubernetes API. Below
is an example, configuration to achieve this.

```yaml
receivers:
  kubeletstats:
    collection_interval: 10s
    auth_type: "serviceAccount"
    endpoint: "${K8S_NODE_NAME}:10250"
    insecure_skip_verify: true
    extra_metadata_labels:
      - k8s.volume.type
    k8s_api_config:
      auth_type: serviceAccount
```

If `k8s_api_config` set, the receiver will attempt to collect metadata from underlying storage resources for
Persistent Volume Claims. For example, if a Pod is using a PVC backed by an EBS instance on AWS, the receiver
would set the `k8s.volume.type` label to be `awsElasticBlockStore` rather than `persistentVolumeClaim`.

### Metric Groups

A list of metric groups from which metrics should be collected. By default, metrics from containers,
pods and nodes will be collected. If `metric_groups` is set, only metrics from the listed groups
will be collected. Valid groups are `container`, `pod`, `node` and `volume`. For example, if you're
looking to collect only `node` and `pod` metrics from the receiver use the following configuration.

```yaml
receivers:
  kubeletstats:
    collection_interval: 10s
    auth_type: "serviceAccount"
    endpoint: "${K8S_NODE_NAME}:10250"
    insecure_skip_verify: true
    metric_groups:
      - node
      - pod
```

### Optional parameters

The following parameters can also be specified:

- `collection_interval` (default = `10s`): The interval at which to collect data.
- `insecure_skip_verify` (default = `false`): Whether or not to skip certificate verification.

The full list of settings exposed for this receiver are documented [here](./config.go)
with detailed sample configurations [here](./testdata/config.yaml).
