# Trace ID aware load-balancing exporter demo for kubernetes service resolver

## How to run

1. Pre-requirements:
```shell
kubectl apply -f https://github.com/open-telemetry/opentelemetry-operator/releases/latest/download/opentelemetry-operator.yaml
```

2. Once the opentelemetry-operator is deployed, create the OpenTelemetry Collector (otelcol) instances, ServiceAccount and other necessary resources, run:
```shell
kubectl apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: observability
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: loadbalancer-role
  namespace: observability
rules:
- apiGroups:
  - ""
  resources:
  - endpoints
  verbs:
  - list
  - watch
  - get
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: loadbalancer
  namespace: observability
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: loadbalancer-rolebinding
  namespace: observability
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: loadbalancer-role
subjects:
- kind: ServiceAccount
  name: loadbalancer
  namespace: observability
---
apiVersion: opentelemetry.io/v1alpha1
kind: OpenTelemetryCollector
metadata:
  name: loadbalancer
  namespace: observability
spec:
  image: docker.io/jpkroehling/otelcol-with-k8sresolver:latest
  serviceAccount: loadbalancer
  config: |
    receivers:
      otlp:
        protocols:
          grpc:

    processors:

    exporters:
      loadbalancing:
        protocol:
          otlp:
            tls:
              insecure: true
        resolver:
          k8s:
            service: backends-collector-headless.observability

    service:
      pipelines:
        traces:
          receivers:
            - otlp
          processors: []
          exporters:
            - loadbalancing
---
apiVersion: opentelemetry.io/v1alpha1
kind: OpenTelemetryCollector
metadata:
  name: backends
  namespace: observability
spec:
  replicas: 5
  config: |
    receivers:
      otlp:
        protocols:
          grpc:

    processors:

    exporters:
      debug:

    service:
      pipelines:
        traces:
          receivers:
            - otlp
          processors: []
          exporters:
            - debug
EOF
```

## How does it work
- The `loadbalancer` and `backends` OpenTelemetryCollector are recognised by the opentelemetry-operator, which then creates the appropriate deployment resources.
- The `loadbalancer` ServiceAccount is used to assign to the pods corresponding to the deployment loadbalancer and backends.
- The `loadbalancer-role` Role grants `get`, `list`, and `watch` permissions on endpoint resources in its namespace. Having these permissions is essential for a kubernetes service resolver to work properly.
- The `loadbalancer-rolebinding` RoleBinding is used to bind Role `loadbalancer-role` to ServiceAccount `loadbalancer`.