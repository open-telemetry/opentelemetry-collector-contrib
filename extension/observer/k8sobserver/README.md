# Kubernetes Observer

The k8sobserver uses the Kubernetes API to discover pods running on the local node. This assumes the collector is deployed in the "agent" model where it is running on each individual node/host instance.

## Config

**auth_type**

How to authenticate to the K8s API server.  This can be one of `none` (for no auth), `serviceAccount` (to use the standard service account token provided to the agent pod), or `kubeConfig` to use credentials from `~/.kube/config`.

**node**

Node should be set to the node name to limit discovered endpoints to. For example, node name can be set using the downward API inside the collector pod spec as follows:

```yaml
env:
  - name: K8S_NODE_NAME
    valueFrom:
      fieldRef:
        fieldPath: spec.nodeName
```

Then set this value to `${K8S_NODE_NAME}` in the configuration.

The full list of settings exposed for this exporter are documented [here](./config.go)
with detailed sample configurations [here](./testdata/config.yaml).

