# Kubelet Stats Receiver

### Overview

The Kubelet Stats Receiver pulls pod metrics from the API server on a kubelet
and sends it down the metric pipeline for further processing.

Status: beta

### Configuration

A kubelet runs on a kubernetes node and has an API server to which this
receiver connects. To configure this receiver, you have to tell it how
to connect and authenticate to the API server and how often to collect data
and send it to the next consumer.

There are two ways to authenticate, driven by the `auth_type` field: "tls" and
"serviceAccount".

TLS tells this receiver to use TLS for auth and requires that the fields
`ca_file`, `key_file`, and `cert_file` also be set.

ServiceAccount tells this receiver to use the default service account token
to authenticate to the kubelet API.

Note: a missing or empty `endpoint` will cause the hostname on which the collector
is running to be used as the endpoint. If the hostNetwork flag is set, and the
collector is running in a pod, this hostname will resolve to the node's network
namespace.

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
kubelet api from a pod, the preferred approach is to use the downard API.
 
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
