# Resource UUID processor

Enriches logs using the local eworker graph and its existing core-synchronized
UUID catalog. The collector never calls core or Kubernetes.

```yaml
processors:
  resourceuuid:
    endpoint: http://127.0.0.1:2301/api/v1/provider/graph?kind=Pod&edges=false&workloadDetails=true
    refresh_interval: 2m
    retry_interval: 2m
    cache_ttl: 10m
    cache_size: 10000
    timeout: 5s
  resourceuuid/node:
    endpoint: http://127.0.0.1:2301/api/v1/provider/graph?kind=Pod&edges=false&workloadDetails=true
    node_logs: true
```

The pod instance matches `k8s.pod.uid`, writes the pod's OpsRamp UUID to
`resourceUUID` and `k8s.pod.resourceUUID`, and retains `k8s.pod.uuid` (or the
configured `target_attribute`) as a compatibility alias. Workload owners map to
`k8s.<kind>.name`, `k8s.<kind>.uid` and `k8s.<kind>.resourceUUID`.
ReplicaSet/Deployment and Job/CronJob identities remain separate. Supported
kinds also include StatefulSet, DaemonSet, ReplicationController and Rollout.

The node instance matches `k8s.node.name` and skips pod-identified logs. Unknown
pods never fall back to their node's UUID. Each instance caches only its resource
type, although both currently fetch the same full pod response.

## Optional graph enrichment

`GET /api/v1/provider/graph?kind=Pod&edges=false&workloadDetails=true` uses the
existing eworker graph API. Omitted or `workloadDetails=false` preserves the
original response and performs no workload-repository reads. Values other than
`true` or `false` receive HTTP 400; enrichment is available only on the eworker
endpoint, not the controller-tier graph.

When enabled, the response adds top-level context and per-pod details:

```json
{
  "meta": {"generatedAtMs": 1791528000000},
  "nodes": [
    {
      "moid": "example_pod-uid",
      "uuid": "pod-resource-uuid",
      "kind": "Pod",
      "name": "application",
      "namespace": "default",
      "workloadDetails": {
        "attributes": {
          "k8s.cluster.name": "example",
          "k8s.node.name": "worker-1",
          "k8s.namespace.name": "default",
          "k8s.pod.name": "application",
          "k8s.pod.uid": "pod-uid",
          "resourceUUID": "pod-resource-uuid",
          "k8s.pod.resourceUUID": "pod-resource-uuid"
        },
        "owners": [
          {
            "kind": "Deployment",
            "name": "application",
            "uid": "deployment-uid",
            "resourceUUID": "deployment-resource-uuid"
          }
        ]
      }
    }
  ],
  "edges": [],
  "workloadDetails": {
    "schemaVersion": 1,
    "clusterName": "example",
    "node": {
      "name": "worker-1",
      "attributes": {
        "k8s.cluster.name": "example",
        "k8s.node.name": "worker-1",
        "resourceUUID": "node-resource-uuid",
        "k8s.node.resourceUUID": "node-resource-uuid"
      }
    }
  }
}
```

The graph's existing fields are unchanged. The top-level context supports node
logs even with no selected pods. Additional attributes include available pod IP,
namespace UID/UUID, node UID/UUID and cluster UUID.

The request takes the graph snapshot, releases its lock, reads only selected
pods' identity/owner fields in one repository batch, then resolves referenced
UUIDs in one graph batch. It does not copy labels/ports, query Kubernetes, mutate
graph nodes or change cloud synchronization state. Graph/repository locks are
never held together. The join is eventually consistent, not a transaction across
stores; mismatched pod UID/name/namespace/node causes that pod's extra details
to be omitted. Its existing graph UUID can still be used.

The shared owner record retains parent UIDs during the existing ReplicaSet/Job
lookup. It is bound to the pod UID; no separate logs-only owner cache is created.
Lookup failure leaves the parent unknown until subsequent pod processing resolves
it. UUIDs are resolved at request time, so catalog updates do not require rewriting
pod metadata. Unknown values are omitted, not fabricated.

## Cache and failure behavior

Refresh is asynchronous and not per log. Records arriving before the first
successful refresh can be exported without enrichment; missing metadata never
holds or drops logs. Kubernetes metadata remains usable without cloud UUIDs.
Incoming attributes not supplied by enrichment are preserved.

Failed/invalid refreshes are logged and preserve the previous cache.
Successful responses add new pods and replace the metadata of cached ones, but never
remove entries: an entry is dropped only after `cache_ttl` without being looked up by
a log record (and by the LRU size cap). Deleted nodes in enriched graph responses are
not added. There is no historical retention for
logs read after pod deletion. Cross-cluster enrichment is rejected.

The producer rejects requests selecting more than 10,000 live pods for
enrichment. The consumer rejects responses over 16 MiB, more than 10,000 pod
entries, unsupported versions, duplicate UIDs and inconsistent identities.
The original graph response without workload details remains accepted for
UUID-only enrichment, and the processor's legacy default endpoint is unchanged.

## CR-generated pipelines and rollout

The renderer opts into `workloadDetails=true`; no CR schema change is needed.
Pod-file logs additionally use `body.message`, `source=kubernetes`, `type=log`,
`resourceName`, and a log-level copy of `k8s.container.name`. Severity is extracted
before wrapping the body. OTLP bodies and custom node parsers are unchanged.

Deploy the updated collector first, then the updated eworker. The new collector
accepts old graph responses; an old collector cannot validate the new `node_logs`
configuration. Both images need rebuilding. Provider observability and pod-watch
prerequisites still apply. No separate metadata endpoint, static fallback or core
HTTP dependency is introduced.
