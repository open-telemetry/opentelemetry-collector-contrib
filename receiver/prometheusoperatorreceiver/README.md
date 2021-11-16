# Simple Prometheus Receiver

The `prometheus_operator` receiver is a wrapper around the [prometheus
receiver](../prometheusreceiver).
This receiver allows to use [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator) 
CRDs like `ServiceMonitor` to configure metric collection.

Supported pipeline types: metrics

> :construction: This receiver is in **development**, and is therefore not usable at the moment.

## Configuration

The following settings are optional:

- `auth_type` (default = `serviceAccount`): Determines how to authenticate to
  the K8s API server. This can be one of `none` (for no auth), `serviceAccount`
  (to use the standard service account token provided to the agent pod), or
  `kubeConfig` to use credentials from `~/.kube/config`.
- `namespaces` (default = `all`): An array of `namespaces` to collect events from.
  This receiver will continuously watch all the `namespaces` mentioned in the array for
  new events.

Examples:

```yaml
  prometheus_operator:
    auth_type: kubeConfig
    namespaces: 
      - default
      - my_namespace
    monitor_selector:
      match_labels:
        prometheus-operator-instance: "cluster"
```

The full list of settings exposed for this receiver are documented [here](./config.go)
with detailed sample configurations [here](./testdata/config.yaml).
