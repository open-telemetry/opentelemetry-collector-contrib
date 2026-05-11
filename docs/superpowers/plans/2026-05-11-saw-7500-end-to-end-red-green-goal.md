# SAW-7500 End-to-End Red/Green Goal

## Copy/paste `/goal`

```text
/goal Follow docs/superpowers/plans/2026-05-11-saw-7500-end-to-end-red-green-goal.md and drive SAW-7500 all the way through release and production validation without deploying to BigID: implement the central-queue concurrency fix in opentelemetry-collector-contrib, prove it with deterministic red/green tests, merge with green PR sweeps, propagate the dependency/version/config through sawmills-collector and collectors-service, update the required LaunchDarkly feature flags with `ldcli`, deploy and test in staging and non-BigID prod, and prove from controlled simulation plus live non-BigID runtime metrics that the original incident shape no longer occurs. End with a BigID-ready deployment package and exact `ldcli`/deployment instructions, but do not apply them to BigID.

The proof must cover:
1. `central_queue.num_consumers` as the single operator-facing send-parallelism knob.
2. Internally derived/effective lane count; `lane_count` is advanced/internal queue sharding, not the main operational knob.
3. Consistent lane hashing for logs and metrics.
4. Memory safety validation: `num_consumers * max_uncompressed_batch_bytes <= max_inflight_uncompressed_bytes`.
5. Boring liveness: backend churn, queue pressure, and exporter drain must not cause kubelet liveness kills.
   Use the existing Sawmills `backend_drain` extension as the Kubernetes `/ready` owner; do not add a second readiness HTTP server inside contrib.
6. Dispatch/queue observability: active consumers, configured consumers, effective lanes, queue bytes/items/age, refusals, rejections, reroutes, quarantine, and backend latency.
7. Evidence-backed request sizing: keep `target_compressed_bytes=256KiB` unless a red/green test proves a different value is better without violating latency/memory gates.
8. A reusable local/manual or nightly regression harness for the `production-mt-4-us-east-1`-shaped load + backend rolling deploy shape.

Done means all touched PRs from contrib through sawmills-collector and collectors-service are merged or explicitly blocked with evidence, CI/review sweeps are green, LaunchDarkly feature-flag changes are applied and read back with `ldcli`, staging and any required non-BigID prod deployments are verified from live runtime/config/metrics, BigID is ready to deploy from a reviewed package without being deployed, and SAW-7500 contains the artifact links and final red/green verdicts.

Keep SAW-7500 updated continuously while working: after each meaningful red/green gate, PR event, CI sweep, merge, release, deploy, blocker, or live validation step, add or update a ticket comment with the current status, evidence links, and next step. The ticket must remain the recovery anchor until the goal is complete.
```

## Intent

The goal is not to tune HPA until the incident disappears. The goal is to remove the limiting factor that caused the cascade:

- queue drain pinned behind slow/stale backend exports,
- backend p99 pinned at timeout,
- central queue pressure causing rejections/refusals,
- LB process missing liveness while under runtime pressure,
- HPA/LB scale-up amplifying symptoms instead of fixing the bottleneck.

The corrected system should treat backend churn as expected turbulence: it keeps liveness alive, bounds memory by bytes, reroutes away from failed endpoints, drains queue work in parallel, and recovers without needing LB replica explosion.

## Red/Green Gates

### 1. Contrib Red

Reproduce or retain evidence for the old behavior:

- queue drain pinned behind a slow backend,
- backend p99 at timeout, for example `5000ms`,
- queue pressure, refused records, or rejected compressed bytes,
- at least one LB liveness-probe kill in the local `production-mt-4-us-east-1`-shaped harness,
- no OOMKilled containers, proving liveness failure is the symptom rather than the root cause.

### 2. Contrib Green

The same local workload passes with the fixed exporter behavior:

- `central_queue.num_consumers` drives send parallelism,
- child OTLP retry disabled when central queue owns retry/backpressure,
- endpoint-local central queue failure reroutes immediately,
- logs and metrics use consistent lane hashing,
- queue remains bounded below configured capacity,
- queue oldest age returns to baseline,
- zero LB liveness restarts,
- zero sustained refusals/rejections,
- settled backend p95 under `2s`,
- p99 not pinned at timeout,
- PR checks and review sweep are green before merge.

### 3. Sawmills-Collector Red

Show current collector release cannot yet provide the fixed behavior:

- missing fixed contrib dependency/version,
- or missing new config surface,
- or local collector smoke cannot render/run the new central queue config cleanly.

This red is a dependency/version/config gap, not a production incident.

### 4. Sawmills-Collector Green

Update and validate the collector:

- bump to fixed contrib dependency/version,
- extend the existing `backend_drain` readiness extension to include pressure-aware `/ready` checks when enabled,
- prove `/ready` returns 503 under queue/inflight/memory/rejection pressure while liveness/healthcheck remains boring,
- build/release collector cleanly,
- run collector-level smoke/regression with the fixed central queue config,
- verify generated collector binary accepts `central_queue.num_consumers`,
- merge with green CI/review sweep,
- do not continue downstream until the PR is swept green or an explicit blocker is documented in SAW-7500,
- verify release/tag/artifact exists.

### 5. Collectors-Service Red

Show current generated config is insufficient:

- cannot express `central_queue.num_consumers`,
- or still emits the ambiguous older knob model,
- or LaunchDarkly/default variation cannot serve the intended config shape.

### 6. Collectors-Service Green

Update and validate config generation and flags:

- generated LB config emits `num_consumers=30`,
- `target_compressed_bytes=256KiB`,
- inflight bytes are coherent with consumer count and batch size,
- no unnecessary `lane_count` exposure for normal operation,
- LaunchDarkly/default variations are updated with `ldcli` and read back correctly,
- PR checks and review sweep are green,
- merge complete.

### 7. mt-4-us-east-1 Simulation Red/Green

Do not deploy to BigID. Reproduce the mt-4 incident shape in a controlled local or staging-isolated environment:

- red uses an old collector image/version from before the fix,
- green uses the new collector image/version with the fixed exporter,
- both use the same simulated `production-mt-4-us-east-1` shape: 2 LB pods, 4 backend workers, backend rolling deploy churn, slow/stale backend injection, 256KiB request target, bounded queue bytes, and constrained LB CPU/memory,
- red must show the historical failure signature: timeout-pinned backend p99, queue pressure, rejected/refused records or delivery mismatch, and LB liveness-probe kill without OOMKilled,
- green must show bounded queue, zero LB liveness restarts, zero sustained refusals/rejections, generated-vs-delivered match, and backend p95/p99 below gate after settle,
- artifacts must include rendered config, old/new collector image versions, analyzer summaries, pod events, pod restart state, and queue/backend latency metrics.

This is the replacement for a BigID canary. BigID stays a source of historical incident evidence only.

### 8. Staging Green

Deploy the fixed chain to staging and verify from runtime:

- deployed collector version includes the fixed exporter,
- rendered live LB config contains `central_queue.num_consumers`,
- pod metrics expose configured consumers, active consumers, and effective lanes,
- no liveness restarts during controlled smoke/load/backend churn,
- no sustained central queue growth,
- zero sustained refusals/rejections,
- backend p95/p99 below gate,
- evidence linked back to SAW-7500.

### 9. Prod Green

Roll out through the intended production mechanism without targeting BigID clusters:

- rendered live config and collector version match the fixed release,
- deployment target is explicitly non-BigID,
- queue bytes/items/oldest age stay bounded,
- zero liveness kills,
- zero sustained refusals/rejections,
- backend latency not pinned at timeout,
- no LB replica explosion,
- evidence linked back to SAW-7500.

### 10. BigID Ready, Not Deployed

Prepare the BigID rollout package after staging and non-BigID prod are green:

- identify the exact BigID clusters that would receive the fix,
- record the collector image/version, collectors-service version, Helm/chart values, and `ldcli` feature-flag patch commands required for BigID,
- read back the current BigID LaunchDarkly targeting/config without mutating it,
- render or dry-run the BigID config and prove it contains the fixed collector version and pressure-safe LB config,
- define the post-deploy validation checklist: collector version, rendered config, `/ready` behavior if enabled, queue bytes/items/oldest age, active/configured consumers, effective lanes, refused/rejected counters, backend p95/p99, LB replica count, pod restarts, and events,
- link the package and dry-run/readback evidence in SAW-7500.

Do not manufacture a new production red. Do not deploy to BigID. Do not update BigID LaunchDarkly targeting/config. BigID evidence is the historical SAW-7500 incident evidence already captured in the ticket plus the mt-4-shaped simulation evidence and the ready-to-deploy dry run.

## Current Local Evidence

Local contrib proof already exists for the concurrency-model follow-up:

- artifact: `artifacts/saw-7500/20260511T051128Z-both-5k-numconsumers30-cpu`,
- red analyzer status: `0`,
- green analyzer status: `0`,
- red: 2 LB liveness-probe kills, 0 OOMKilled containers, p95/p99 pinned at `5000ms`,
- green: 0 LB restarts, 0 liveness kills, 0 refusals, 0 rejections,
- green settled p95 `245.3ms`, settled p99 `486.4ms`,
- generated and delivered logs both `5,164,110`,
- dispatch evidence: configured consumers aggregate `60` across 2 LB pods, active consumers up to `43`, effective lanes aggregate `128`.

## Ticket Update Rule

Keep SAW-7500 as the recovery anchor:

- keep updating the ticket as work progresses until this goal is complete,
- update it after each red/green boundary,
- update it after each PR, CI sweep, merge, release, deploy, blocker, and live-validation step,
- include artifact paths, PR links, run IDs, version/tag values, `ldcli` LaunchDarkly patches/readbacks, deployment targets, BigID dry-run package links, and live metric snapshots,
- separate completed evidence from remaining blockers,
- do not call the issue fixed until prod green evidence is present or an explicit external blocker is documented.
