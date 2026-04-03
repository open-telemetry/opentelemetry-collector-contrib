# Shared Informer Cache Manual Validation

`processor.k8sattributes.ShareInformerCaches` enables same-config sharing of the processor's watcher/cache-backed Kubernetes client across signal-specific processor instances in the same Collector process.

Phase 1 is intentionally conservative (keep it simple and low risk):

- Sharing is enabled by default only when the feature gate is enabled.
- Only processor instances with strictly equivalent defaulted configuration share the same watcher/cache instance.
- `passthrough: true` always bypasses shared watcher/ informer cache creation.

This gate only covers `k8sattributesprocessor` same-config sharing. It does not provide a generic shared Kubernetes cache for other components.

## Manual Validation

The repository includes a shared base plus runnable overlays for the real-cluster validation flow under:

- `processor/k8sattributesprocessor/testdata/manual-validation/shareinformercaches/base`
- `processor/k8sattributesprocessor/testdata/manual-validation/shareinformercaches/feature-gate-on`
- `processor/k8sattributesprocessor/testdata/manual-validation/shareinformercaches/feature-gate-off`

### Feature Gate On

Apply the feature-gate-on overlay and wait for the collector plus telemetry generators to be ready:

```shell
kubectl apply -k processor/k8sattributesprocessor/testdata/manual-validation/shareinformercaches/feature-gate-on
kubectl rollout status deployment/otelcol-shared-watch-manual -n k8sattributes-shared-manual
kubectl rollout status deployment/telemetrygen-manual-traces -n k8sattributes-shared-manual
kubectl rollout status deployment/telemetrygen-manual-metrics -n k8sattributes-shared-manual
kubectl rollout status deployment/telemetrygen-manual-logs -n k8sattributes-shared-manual
```

Check the collector-side watcher evidence. The required result is exactly one shared watcher startup line and exactly one shared cache instance identifier reused by all three processor instances:

```shell
kubectl logs -n k8sattributes-shared-manual deploy/otelcol-shared-watch-manual | rg 'shared k8sattributes watch client|starting dedicated k8sattributes watch client'
```

Check enrichment output in the collector logs:

```shell
kubectl logs -n k8sattributes-shared-manual deploy/otelcol-shared-watch-manual | rg 'manual-(traces|metrics|logs)-deployment|k8s.namespace.name|k8s.pod.uid|k8s.deployment.name'
```

Capture kube-apiserver metrics before and after startup and compare the deltas for watch-related metrics:

```shell
kubectl get --raw /metrics | rg 'apiserver_(registered_watchers|longrunning_requests).*resource="(pods|namespaces|nodes|replicasets|deployments|statefulsets|daemonsets|jobs)"'
```

If audit logging is enabled, the required audit-log check is that the collector service account shows exactly one `watch` stream per watched resource type in the feature-gate-on profile:

```shell
kubectl logs -n kube-system kube-apiserver-kind-control-plane | rg 'system:serviceaccount:k8sattributes-shared-manual:otelcol-shared-watch-manual.*watch'
```

Acceptance for `feature-gate-on`:

- Exactly one `starting shared k8sattributes watch client` log entry.
- Exactly one unique `shared_cache_instance_id` across all `using shared k8sattributes watch client` log entries.
- No `starting dedicated k8sattributes watch client` entries for the equivalent traces/metrics/logs processors.
- Enriched traces, metrics, and logs still contain Kubernetes metadata.

Clean up:

```shell
kubectl delete -k processor/k8sattributesprocessor/testdata/manual-validation/shareinformercaches/feature-gate-on --ignore-not-found
kubectl delete namespace k8sattributes-shared-manual --ignore-not-found
kubectl get namespace k8sattributes-shared-manual >/dev/null 2>&1 && kubectl wait --for=delete namespace/k8sattributes-shared-manual --timeout=120s
```

### Feature Gate Off

Apply the feature-gate-off overlay and wait for the collector plus telemetry generators to be ready:

```shell
kubectl apply -k processor/k8sattributesprocessor/testdata/manual-validation/shareinformercaches/feature-gate-off
kubectl rollout status deployment/otelcol-shared-watch-manual -n k8sattributes-shared-manual
kubectl rollout status deployment/telemetrygen-manual-traces -n k8sattributes-shared-manual
kubectl rollout status deployment/telemetrygen-manual-metrics -n k8sattributes-shared-manual
kubectl rollout status deployment/telemetrygen-manual-logs -n k8sattributes-shared-manual
```

Collector log evidence should now show dedicated startup lines instead of a shared watcher startup:

```shell
kubectl logs -n k8sattributes-shared-manual deploy/otelcol-shared-watch-manual | rg 'shared k8sattributes watch client|starting dedicated k8sattributes watch client'
```

The required comparison against `feature-gate-on` is:

- Higher watch-stream counts per resource type in kube-apiserver metrics and audit logs.
- Dedicated startup log count matching the number of configured processor instances.
- No shared cache instance reuse logs.

Clean up:

```shell
kubectl delete -k processor/k8sattributesprocessor/testdata/manual-validation/shareinformercaches/feature-gate-off --ignore-not-found
kubectl delete namespace k8sattributes-shared-manual --ignore-not-found
kubectl get namespace k8sattributes-shared-manual >/dev/null 2>&1 && kubectl wait --for=delete namespace/k8sattributes-shared-manual --timeout=120s
```
