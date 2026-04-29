# Resource Attribute Mapping

This document describes Prometheus receiver mapping from scrape target metadata and
service discovery meta-labels to OpenTelemetry resource attributes.

## General mappings

- `job` -> `service.name`
- `instance` -> `service.instance.id`
- `instance` host -> `server.address` (when the host is present and discernible)
- `instance` port -> `server.port` (when the port is present)
- `__scheme__` -> `url.scheme`

## Kubernetes service discovery mappings

When Kubernetes service discovery metadata is present, the receiver maps:

- `__meta_kubernetes_pod_name` -> `k8s.pod.name`
- `__meta_kubernetes_pod_uid` -> `k8s.pod.uid`
- `__meta_kubernetes_pod_container_name` -> `k8s.container.name`
- `__meta_kubernetes_namespace` -> `k8s.namespace.name`
- `__meta_kubernetes_pod_node_name` -> `k8s.node.name`
- `__meta_kubernetes_node_name` -> `k8s.node.name`
- `__meta_kubernetes_endpoint_node_name` -> `k8s.node.name`

When both `__meta_kubernetes_pod_controller_kind` and
`__meta_kubernetes_pod_controller_name` are present, the receiver sets one of
the following attributes based on controller kind, using controller name as the
attribute value:

- `k8s.replicaset.name`
- `k8s.daemonset.name`
- `k8s.statefulset.name`
- `k8s.job.name`
- `k8s.cronjob.name`

## Notes

- Mappings are added incrementally and currently cover selected service
  discovery metadata.
- Additional mappings should follow OpenTelemetry semantic conventions.
- The implementation for these mappings is in
  [`internal/prom_to_otlp.go`](./internal/prom_to_otlp.go).
