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
`ca_cert_path`, `client_key_path`, and `client_cert_path` also be set.

ServiceAccount tells this receiver to use the default service account token
to authenticate to the kubelet API.

```yaml
receivers:
  kubeletstats:
    collection_interval: 20s
    auth_type: "tls"
    ca_cert_path: "/path/to/ca.crt"
    client_key_path: "/path/to/apiserver.key"
    client_cert_path: "/path/to/apiserver.crt"
    base_url: "https://192.168.64.1:10250"
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

### Metrics

**pod/ephemeral_storage/capacity**
Represents the storage space available (bytes) for the filesystem. This value is
a combination of total filesystem usage for the containers and emptyDir-backed
volumes in the measured Pod.

See more about emptyDir-backed volumes:
https://kubernetes.io/docs/concepts/storage/volumes/#emptydir

Type: gauge

**pod/ephemeral_storage/used**
Represents the bytes used on the filesystem. This value is a total filesystem
usage for the containers and emptyDir-backed volumes in the measured Pod.

See more about emptyDir-backed volumes:
https://kubernetes.io/docs/concepts/storage/volumes/#emptydir

Type: counter
