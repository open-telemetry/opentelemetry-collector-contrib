# Kubernetes Cluster Receiver

**Status: beta**

### Overview

The Kubernetes Cluster receiver collects cluster-level metrics from the Kubernetes 
API server. It uses the K8s API to listen for updates. A single instance of this 
receiver can be used to monitor a cluster.

Currently this receiver supports authentication via service accounts only. See [example](#example) 
for more information.

### Config

An example config,

```yaml
  k8s_cluster:
    auth_type: kubeConfig
    node_conditions_to_report: [Ready, MemoryPressure]
```

#### auth_type

Determines how to authenticate to the K8s API server.  This can be one of
`none` (for no auth), `serviceAccount` (to use the standard service account
token provided to the agent pod), or `kubeConfig` to use credentials
from `~/.kube/config`.

default: `serviceAccount`

#### collection_interval

This receiver continuously watches for events using K8s API. However, the metrics
collected are emitted only once every collection interval. `collection_interval` will
determine the frequency at which metrics are emitted by this receiver.

default: `10s`

#### node_conditions_to_report

An array of node conditions this receiver should report. See [here](https://kubernetes.io/docs/concepts/architecture/nodes/#condition)
for list of node conditions. The receiver will emit one metric per entry in the
array.

```yaml
...
k8s_cluster:
  node_conditions_to_report:
    - Ready
    - MemoryPressure
...
```

For example, with the above config the receiver will emit two metrics
`kubernetes/node/condition_ready` and `kubernetes/node/condition_memory_pressure`, one
for each condition in the config. The value will be `1` if the `ConditionStatus` for the
corresponding `Condition` is `True`, `0` if it is `False` and -1 if it is `Unknown`.


default: `[Ready]`

#### metadata_exporters

A list of metadata exporters to which metadata being collected by this receiver
should be synced. Exporters specified in this list are expected to implement the
following interface. If an exporter that does not implement the interface is listed,
startup will fail.

```yaml
type KubernetesMetadataExporter interface {
  ConsumeKubernetesMetadata(metadata []*KubernetesMetadataUpdate) error
}

type KubernetesMetadataUpdate struct {
  ResourceIDKey string
  ResourceID    ResourceID
  MetadataDelta
}

type MetadataDelta struct {
  MetadataToAdd    map[string]string
  MetadataToRemove map[string]string
  MetadataToUpdate map[string]string
}
```

See [here](collection/metadata.go) for details about the above types.

### Example

Here is an example deployment of the collector that sets up this receiver along with 
the [SignalFx Exporter](../../exporter/signalfxexporter/README.md).
 
Follow the below sections to setup various Kubernetes resources required for the deployment.

#### Config

Create a ConfigMap with the config for `otelcontribcol`. Replace `SIGNALFX_TOKEN` and `SIGNAL_INGEST_URL` 
with valid values.

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: otelcontribcol
  labels:
    app: otelcontribcol
data:
  config.yaml: |
    receivers:
      k8s_cluster:
        collection_interval: 10s
    exporters:
      signalfx:
        access_token: <SIGNALFX_TOKEN>
        ingest_url: <SIGNALFX_INGEST_URL>

    service:
      pipelines:
        metrics:
          receivers: [k8s_cluster]
          exporters: [signalfx]
EOF
```

#### Service Account

Create a service account that the collector should use.

```bash
<<EOF | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  labels:
    app: otelcontribcol
  name: otelcontribcol
EOF
```

#### RBAC

Use the below commands to create a `ClusterRole` with required permissions and a 
`ClusterRoleBinding` to grant the role to the service account created above.

```bash
<<EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: otelcontribcol
  labels:
    app: otelcontribcol
rules:
- apiGroups:
  - ""
  resources:
  - events
  - namespaces
  - namespaces/status
  - nodes
  - nodes/spec
  - pods
  - pods/status
  - replicationcontrollers
  - replicationcontrollers/status
  - resourcequotas
  - services
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - apps
  resources:
  - daemonsets
  - deployments
  - replicasets
  - statefulsets
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - extensions
  resources:
  - daemonsets
  - deployments
  - replicasets
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - batch
  resources:
  - jobs
  - cronjobs
  verbs:
  - get
  - list
  - watch
- apiGroups:
    - autoscaling
  resources:
    - horizontalpodautoscalers
  verbs:
    - get
    - list
    - watch
EOF
```

```bash
<<EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: otelcontribcol
  labels:
    app: otelcontribcol
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: otelcontribcol
subjects:
- kind: ServiceAccount
  name: otelcontribcol
  namespace: default
EOF
```

#### Deployment

Create a [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/) to deploy the collector.

```bash
<<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: otelcontribcol
  labels:
    app: otelcontribcol
spec:
  replicas: 1
  selector:
    matchLabels:
      app: otelcontribcol
  template:
    metadata:
      labels:
        app: otelcontribcol
    spec:
      serviceAccountName: otelcontribcol
      containers:
      - name: otelcontribcol
        image: otelcontribcol:latest # specify image
        args: ["--config", "/etc/config/config.yaml"]
        volumeMounts:
        - name: config
          mountPath: /etc/config
        imagePullPolicy: IfNotPresent
      volumes:
        - name: config
          configMap:
            name: otelcontribcol
EOF
```