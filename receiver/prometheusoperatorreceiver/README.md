# Simple Prometheus Receiver

The `prometheus_operator` receiver is a wrapper around the [prometheus
receiver](../prometheusreceiver).
This receiver allows using [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator) 
CRDs like `ServiceMonitor` and `PodMonitor` to configure metric collection.

Supported pipeline types: metrics

> :construction: This receiver is in **development**, and is therefore not usable at the moment.

## When to use this receiver?
Choose this receiver, if you use [ServiceMonitor](https://github.com/prometheus-operator/prometheus-operator/blob/main/example/user-guides/getting-started/example-app-service-monitor.yaml) 
or [PodMonitor](https://github.com/prometheus-operator/prometheus-operator/blob/main/example/user-guides/getting-started/example-app-pod-monitor.yaml) 
custom resources in your Kubernetes cluster. These can be provided by applications (e.g. helm charts) or manually 
deployed by users. 

In every other case other Prometheus receivers should be used. 
Below you can find a short description of the available options.

### Prometheus scrape annotations
If you use an annotations based scrape configs like `prometheus.io/scrape = true`, then you can use the
[receivercreator](../receivercreator/README.md). This receiver is able to create multiple receivers based on a set of
rules, which are defined by the user.

A guide how to use this meta receiver with Prometheus annotations you can find in the
[examples of the receivercreator](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/receivercreator#examples)

### Static targets
If a simple static endpoint in or outside the cluster should be scraped, use the [simpleprometheusreceiver](../simpleprometheusreceiver/README.md).
It provides a simplified interface around the `prometheusreceiver`. Use cases could be the federation of Prometheus
instances or scraping of targets outside dynamic setups. 

### Prometheus service discovery and manual configuration
The [prometheusreceiver](../prometheusreceiver/README.md) allows to configure the collector much a like a Prometheus 
server instance and supports the most low-level configuration options for Prometheus metric scraping by the 
OpenTelemetry Collector. Prometheus supports here static configurations as well as dynamic configuration based on the 
service discovery concept. These service discovery options allow to use a multitude of external systems to discovery 
new services. One of them is the 
[Kubernetes API](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config) other options
include public and private cloud provider APIs, the Docker daemon and generic service discovery sources like 
[HTTP_SD](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#http_sd_config) and 
[FILE_SD](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#file_sd_config).

### OpenTelemetry operator
The OpenTelemetry community provides an operator which is the recommended method to operate OpenTelemetry on Kubernetes.
This operator supports deploying the `prometheusoperatorreceiver` for usage in your cluster.

//TODO add target allocator capabilities 

Alternatively targeting methods like the [target allocator](https://github.com/open-telemetry/opentelemetry-operator/tree/main/cmd/otel-allocator)
are in development, but do not support PrometheusOperator CRDs at this time.

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
