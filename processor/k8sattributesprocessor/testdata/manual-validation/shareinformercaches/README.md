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

Both overlays reuse the same `base` manifests. The only intended collector config difference is that `feature-gate-on` adds `--feature-gates=processor.k8sattributes.ShareInformerCaches`.

The base scenario intentionally stresses same-config reuse:

- 9 same-config `k8sattributes` processor instances (3 traces pipelines, 3 metrics pipelines, 3 logs pipelines).
- Low-noise exporter fanout: one `debug/validation` pipeline per signal plus `debug/basic` on the extra stress pipelines.
- Telemetry from Deployment-, StatefulSet-, DaemonSet-, and Job-backed pods so more watched resource kinds are exercised.

### Feature Gate On

Apply the feature-gate-on overlay and wait for the collector plus telemetry generators to be ready:

```shell
kubectl apply -k processor/k8sattributesprocessor/testdata/manual-validation/shareinformercaches/feature-gate-on
kubectl rollout status deployment/otelcol-shared-watch-manual -n k8sattributes-shared-manual
kubectl rollout status deployment/telemetrygen-manual-traces -n k8sattributes-shared-manual
kubectl rollout status deployment/telemetrygen-manual-metrics -n k8sattributes-shared-manual
kubectl rollout status deployment/telemetrygen-manual-logs -n k8sattributes-shared-manual
kubectl rollout status statefulset/telemetrygen-manual-traces-statefulset -n k8sattributes-shared-manual
kubectl rollout status daemonset/telemetrygen-manual-traces-daemonset -n k8sattributes-shared-manual
kubectl get pods -n k8sattributes-shared-manual -l app=telemetrygen-manual-traces-job
```

The `Job` adds `k8s.job.name` coverage. Confirm that its pod appears in the `kubectl get pods` output before continuing.

Increase pod cardinality before measuring memory so duplicated caches are more likely to show up in the whole-collector metrics:

```shell
kubectl scale deployment telemetrygen-manual-traces -n k8sattributes-shared-manual --replicas=10
kubectl scale deployment telemetrygen-manual-metrics -n k8sattributes-shared-manual --replicas=10
kubectl scale deployment telemetrygen-manual-logs -n k8sattributes-shared-manual --replicas=10
kubectl scale statefulset telemetrygen-manual-traces-statefulset -n k8sattributes-shared-manual --replicas=10
kubectl rollout status deployment/telemetrygen-manual-traces -n k8sattributes-shared-manual
kubectl rollout status deployment/telemetrygen-manual-metrics -n k8sattributes-shared-manual
kubectl rollout status deployment/telemetrygen-manual-logs -n k8sattributes-shared-manual
kubectl rollout status statefulset/telemetrygen-manual-traces-statefulset -n k8sattributes-shared-manual
```

Capture a steady-state whole-collector memory baseline for later comparison. Start a port-forward in one terminal:

```shell
kubectl port-forward -n k8sattributes-shared-manual deploy/otelcol-shared-watch-manual 8888:8888
```

In another terminal, run this command three times about 30 seconds apart and record both metrics as the `feature-gate-on` baseline:

```shell
curl -s http://127.0.0.1:8888/metrics | rg '^(otelcol_process_memory_rss|otelcol_process_runtime_heap_alloc_bytes) '
```

Start a second port-forward for `pprof`, then capture a GC-stabilized heap profile for attribution:

```shell
kubectl port-forward -n k8sattributes-shared-manual deploy/otelcol-shared-watch-manual 1777:1777
curl -sS "http://127.0.0.1:1777/debug/pprof/heap?gc=1" -o /tmp/feature-gate-on-heap.pb.gz
go tool pprof -top /tmp/feature-gate-on-heap.pb.gz
```

The heap profile is the preferred signal when RSS and heap metrics disagree because it shows retained Go allocations after a forced GC.

Check the collector-side watcher evidence. The required result is exactly one shared watcher startup line and exactly one shared cache instance identifier reused by all nine processor instances:

```shell
kubectl logs -n k8sattributes-shared-manual deploy/otelcol-shared-watch-manual | rg 'shared k8sattributes watch client|starting dedicated k8sattributes watch client'
```

Check enrichment output in the collector logs:

```shell
kubectl logs -n k8sattributes-shared-manual deploy/otelcol-shared-watch-manual | rg 'manual-(traces|metrics|logs)-(deployment|statefulset|daemonset|job)|k8s\.(namespace\.name|pod\.uid|deployment\.name|statefulset\.name|daemonset\.name|job\.name)'
```

Capture kube-apiserver metrics before and after startup and compare the deltas for watch-related metrics:

```shell
kubectl get --raw /metrics | rg 'apiserver_(registered_watchers|longrunning_requests).*resource="(pods|namespaces|nodes|replicasets|deployments|statefulsets|daemonsets|jobs)"'
```

If audit logging is enabled, the required audit-log check is that the collector service account shows exactly one `watch` stream per watched resource type in the feature-gate-on profile, even though nine same-config processor instances are configured:

```shell
kubectl logs -n kube-system kube-apiserver-kind-control-plane | rg 'system:serviceaccount:k8sattributes-shared-manual:otelcol-shared-watch-manual.*watch'
```

Acceptance for `feature-gate-on`:

- Exactly one `starting shared k8sattributes watch client` log entry.
- Exactly nine `using shared k8sattributes watch client` log entries and exactly one unique `shared_cache_instance_id` across them.
- No `starting dedicated k8sattributes watch client` entries for the nine equivalent traces/metrics/logs processors.
- The validation pipelines still show Kubernetes metadata for deployment-, statefulset-, daemonset-, and job-backed telemetry.
- Pod cardinality is increased to the same replica count that will be used in `feature-gate-off`.
- Steady-state `otelcol_process_memory_rss` and `otelcol_process_runtime_heap_alloc_bytes` baselines are recorded for comparison with `feature-gate-off`.
- A `feature-gate-on` heap profile is captured with `pprof` for later comparison against `feature-gate-off`.

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
kubectl rollout status statefulset/telemetrygen-manual-traces-statefulset -n k8sattributes-shared-manual
kubectl rollout status daemonset/telemetrygen-manual-traces-daemonset -n k8sattributes-shared-manual
kubectl get pods -n k8sattributes-shared-manual -l app=telemetrygen-manual-traces-job
```

The `Job` adds `k8s.job.name` coverage. Confirm that its pod appears in the `kubectl get pods` output before continuing.

Increase pod cardinality to the same replica count used in `feature-gate-on` before taking the comparison measurements:

```shell
kubectl scale deployment telemetrygen-manual-traces -n k8sattributes-shared-manual --replicas=10
kubectl scale deployment telemetrygen-manual-metrics -n k8sattributes-shared-manual --replicas=10
kubectl scale deployment telemetrygen-manual-logs -n k8sattributes-shared-manual --replicas=10
kubectl scale statefulset telemetrygen-manual-traces-statefulset -n k8sattributes-shared-manual --replicas=10
kubectl rollout status deployment/telemetrygen-manual-traces -n k8sattributes-shared-manual
kubectl rollout status deployment/telemetrygen-manual-metrics -n k8sattributes-shared-manual
kubectl rollout status deployment/telemetrygen-manual-logs -n k8sattributes-shared-manual
kubectl rollout status statefulset/telemetrygen-manual-traces-statefulset -n k8sattributes-shared-manual
```

Capture the same steady-state whole-collector memory measurements for comparison with the `feature-gate-on` baseline. Start a port-forward in one terminal:

```shell
kubectl port-forward -n k8sattributes-shared-manual deploy/otelcol-shared-watch-manual 8888:8888
```

In another terminal, run this command three times about 30 seconds apart and compare both metrics against the `feature-gate-on` baseline:

```shell
curl -s http://127.0.0.1:8888/metrics | rg '^(otelcol_process_memory_rss|otelcol_process_runtime_heap_alloc_bytes) '
```

Start a second port-forward for `pprof`, then capture the matching GC-stabilized heap profile:

```shell
kubectl port-forward -n k8sattributes-shared-manual deploy/otelcol-shared-watch-manual 1777:1777
curl -sS "http://127.0.0.1:1777/debug/pprof/heap?gc=1" -o /tmp/feature-gate-off-heap.pb.gz
go tool pprof -top /tmp/feature-gate-off-heap.pb.gz
```

Collector log evidence should now show dedicated startup lines instead of a shared watcher startup:

```shell
kubectl logs -n k8sattributes-shared-manual deploy/otelcol-shared-watch-manual | rg 'shared k8sattributes watch client|starting dedicated k8sattributes watch client'
```

The required comparison against `feature-gate-on` is:

- Higher watch-stream counts per resource type in kube-apiserver metrics and audit logs, including `pods`, `replicasets`, `deployments`, `statefulsets`, `daemonsets`, and `jobs`.
- Dedicated startup log count matching the 9 configured processor instances.
- No shared cache instance reuse logs.
- The same pod cardinality is used as in `feature-gate-on`.
- Steady-state `otelcol_process_memory_rss` and `otelcol_process_runtime_heap_alloc_bytes` are recorded and compared with the `feature-gate-on` baseline.
- Watcher-count reduction remains the primary proof that sharing is working even if whole-process memory stays noisy.
- A `feature-gate-off` heap profile is captured so it can be diffed against `feature-gate-on`.

Clean up:

```shell
kubectl delete -k processor/k8sattributesprocessor/testdata/manual-validation/shareinformercaches/feature-gate-off --ignore-not-found
kubectl delete namespace k8sattributes-shared-manual --ignore-not-found
kubectl get namespace k8sattributes-shared-manual >/dev/null 2>&1 && kubectl wait --for=delete namespace/k8sattributes-shared-manual --timeout=120s
```

## Latest Local Results

Latest local run against the expanded manual-validation topology (9 same-config pipelines plus Deployment/StatefulSet/DaemonSet/Job workloads):

| Scenario | Same-config processor instances | Deployment replicas | StatefulSet replicas | Pod watch delta | ReplicaSet watch delta | Deployment watch delta | StatefulSet watch delta | DaemonSet watch delta | Job watch delta | `pprof` report | Notes |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- | --- |
| `feature-gate-on` | 9 | 10 | 10 | 1 | 1 | 0 | 0 | 0 | 0 | Baseline shared-mode heap profile. Focused profile did not show duplicated dedicated-watcher startup allocations. | `apiserver_longrunning_requests`; 1 shared start, 9 shared-use logs, 1 shared cache id |
| `feature-gate-off` | 9 | 10 | 10 | 9 | 9 | 0 | 0 | 0 | 0 | Focused diff vs `feature-gate-on` retained extra watcher-related heap, led by `internal/kube.removeUnnecessaryPodData` (~1.0 MiB) and `golang.org/x/time/rate.NewLimiter` (~0.5 MiB). | `apiserver_longrunning_requests`; 9 dedicated starts, no shared reuse |

Whole-process memory context from this run:

- `feature-gate-on`: RSS avg `239.2 MiB`, heap avg `52.1 MiB`
- `feature-gate-off`: RSS avg `247.6 MiB`, heap avg `86.8 MiB`

If RSS and heap still disagree, compare the captured heap profiles directly:

```shell
go tool pprof -top -base /tmp/feature-gate-on-heap.pb.gz /tmp/feature-gate-off-heap.pb.gz
go tool pprof -top -base /tmp/feature-gate-off-heap.pb.gz /tmp/feature-gate-on-heap.pb.gz
```

To isolate watcher and informer-cache-retained heap from general Collector traffic and exporter buffers, focus the diff on the Kubernetes-related frames:

```shell
go tool pprof -top -focus 'k8sattributesprocessor|k8s.io/client-go/tools/cache|k8s.io/apimachinery/pkg/watch|k8s.io/client-go/rest/watch|k8s.io/utils/buffer|internal/kube' -base /tmp/feature-gate-on-heap.pb.gz /tmp/feature-gate-off-heap.pb.gz
go tool pprof -top -focus 'k8sattributesprocessor|k8s.io/client-go/tools/cache|k8s.io/apimachinery/pkg/watch|k8s.io/client-go/rest/watch|k8s.io/utils/buffer|internal/kube' -base /tmp/feature-gate-off-heap.pb.gz /tmp/feature-gate-on-heap.pb.gz
```

Interpretation:

- Use the watch deltas to confirm the mechanism: shared mode should open fewer watches for the same config.
- Treat RSS and `otelcol_process_runtime_heap_alloc_bytes` as auxiliary whole-collector context, not as the primary summary result for watcher/cache sharing.
- Use the focused `pprof` diff to judge watcher/cache memory specifically; the unfocused profile will often be dominated by OTLP receiver, gRPC, and debug-exporter buffers.
- If the heap profile is lower in shared mode but RSS is flat or slightly higher, treat that as allocator or OS residency noise unless the retained heap diff also points to larger shared-mode objects.
- If the memory difference is still noisy, increase pod cardinality further or add more workloads before drawing a conclusion.
