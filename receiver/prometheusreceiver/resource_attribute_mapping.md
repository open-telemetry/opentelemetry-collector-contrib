# Resource Attribute Mapping

This document describes Prometheus receiver mapping from scrape target metadata and
service discovery meta-labels to OpenTelemetry resource attributes.

## General mappings

| Prometheus source | OpenTelemetry resource attribute | Notes |
| --- | --- | --- |
| `job` | `service.name` | |
| `instance` | `service.instance.id` | |
| `instance` (host) | `server.address` | When the host is present and discernible |
| `instance` (port) | `server.port` | When the port is present |
| `__scheme__` | `url.scheme` | |

## Kubernetes service discovery mappings

When Kubernetes service discovery metadata is present, the receiver maps:

| Prometheus source | OpenTelemetry resource attribute | Notes |
| --- | --- | --- |
| `__meta_kubernetes_pod_name` | `k8s.pod.name` | |
| `__meta_kubernetes_pod_uid` | `k8s.pod.uid` | |
| `__meta_kubernetes_pod_container_name` | `k8s.container.name` | |
| `__meta_kubernetes_namespace` | `k8s.namespace.name` | |
| `__meta_kubernetes_pod_node_name` | `k8s.node.name` | Only one of `__meta_kubernetes_pod_node_name`, `__meta_kubernetes_node_name`, and `__meta_kubernetes_endpoint_node_name` will be present for a target |
| `__meta_kubernetes_node_name` | `k8s.node.name` | Same as above |
| `__meta_kubernetes_endpoint_node_name` | `k8s.node.name` | Same as above |

When both `__meta_kubernetes_pod_controller_kind` and
`__meta_kubernetes_pod_controller_name` are present, the receiver sets one of
the following attributes based on controller kind, using controller name as the
attribute value:

| Controller kind (`__meta_kubernetes_pod_controller_kind`) | OpenTelemetry resource attribute |
| --- | --- |
| `ReplicaSet` | `k8s.replicaset.name` |
| `DaemonSet` | `k8s.daemonset.name` |
| `StatefulSet` | `k8s.statefulset.name` |
| `Job` | `k8s.job.name` |
| `CronJob` | `k8s.cronjob.name` |

## Notes

- Mappings are added incrementally and currently cover selected service
  discovery metadata.
- Additional mappings should follow OpenTelemetry semantic conventions.
- The implementation for these mappings is in
  [`internal/prom_to_otlp.go`](./internal/prom_to_otlp.go).
