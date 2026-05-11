# SAW-7500 Hardening Evidence

## Verdict

The queue/backpressure fix path is green in the local mt-4-shaped harness. The
old collector shows the limiting factor under the same shape: backend latency
pins at the 5s timeout and queue age grows. The fixed collector keeps liveness
boring, drains the queue, delivers all generated logs, and keeps settled backend
latency below the 2s gate.

Keep `central_queue.target_compressed_bytes=262144` for now. The 1 MiB target
passes, but it does not improve latency, delivery, liveness, refusals, or queue
drain, and it increases max inflight uncompressed bytes versus 256 KiB.

Do not implement a `/live` and `/ready` split in this workstream yet. The local
evidence shows stable liveness and clean drain after the actual queue/backpressure
fix. Treat the split as defense-in-depth and reopen it only if a green harness
run shows liveness risk while the process is otherwise draining.

No BigID deployment was performed.

## Main Red/Green Proof

Artifacts: `artifacts/saw-7500/evidence-1mib-20260511T151922Z`.

Shape:

- 2 LB pods, 4 backend workers.
- Old red image: `public.ecr.aws/s7a5m1b4/sawmills-collector:1.936.0`.
- Green image: `otelcontribcol-dev:saw-7500`.
- `payload_profile=repeated`, `payload_size_bytes=1048576`.
- `target_compressed_bytes=262144`.
- `num_consumers=30`.
- 300s load, 60s warmup, 90s settle.
- Backend slow window: 180s, 6s sleep, 5s backend timeout.

| phase | verdict | generated | delivered | max age ms | inflight MiB | RSS MiB | settled p95 ms | settled p99 ms | settled >2s | live kills | restarts | rejected | refused |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| red old collector | red signature | 15000 | 15000 | 160959 | 320.0 | 1585.2 | 49 | 5000 | 8 | 0 | 0 | 0 | 0 |
| green fixed collector | pass | 15000 | 15000 | 238 | 80.0 | 1231.8 | 48 | 50 | 0 | 0 | 0 | 0 | 0 |

Interpretation:

- Red proves the upstream limiting factor: timeout-pinned backend exports plus
  queue-age growth. It does not need a local kubelet kill to be useful because
  liveness death was the downstream production symptom.
- Green proves the fix path: stable liveness, generated equals delivered, queue
  oldest age returns to baseline, no refusals/rejections, and settled latency is
  well below the 2s gate.

## Large Compressible Payload Coverage

Additional 256 KiB body proof artifact:
`artifacts/saw-7500/evidence-256k-green-20260511T1633Z`.

| payload body | phase | target | verdict | generated | delivered | max age ms | inflight MiB | RSS MiB | settled p95 ms | settled p99 ms | settled >2s | live kills | restarts | rejected | refused |
|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 256 KiB | green | 256 KiB | pass | 15000 | 15000 | 218 | 20.0 | 742.0 | 19 | 20 | 0 | 0 | 0 | 0 | 0 |
| 1 MiB | green | 256 KiB | pass | 15000 | 15000 | 238 | 80.0 | 1231.8 | 48 | 50 | 0 | 0 | 0 | 0 | 0 |

This isolates the high-compression memory-strain hypothesis enough for the
current fix: larger uncompressed bodies increase inflight/RSS pressure, but the
fixed collector remains responsive and drains cleanly under the mt-4-shaped
backend churn.

## Request-Size Sweep

Artifacts: `artifacts/saw-7500/sweep-1mib-long-20260511T160455Z`.

Payload body: 1 MiB repeated/highly-compressible logs. All targets used the same
load, backend churn, 2 LB pods, 4 workers, and `num_consumers=30`.

| target_compressed_bytes | size | pass | p95 ms | p99 ms | over2s | RSS MiB | inflight MiB | age ms | rejected | refused | generated=delivered | live kills | restarts |
|---:|---:|:---:|---:|---:|---:|---:|---:|---:|---:|---:|:---:|---:|---:|
| 131072 | 128 KiB | yes | 48 | 50 | 0 | 1184.4 | 60.0 | 221 | 0 | 0 | yes | 0 | 0 |
| 262144 | 256 KiB | yes | 48 | 50 | 0 | 1183.4 | 70.0 | 246 | 0 | 0 | yes | 0 | 0 |
| 524288 | 512 KiB | yes | 48 | 50 | 0 | 1188.7 | 90.0 | 247 | 0 | 0 | yes | 0 | 0 |
| 1048576 | 1 MiB | yes | 49 | 50 | 0 | 1166.5 | 110.0 | 251 | 0 | 0 | yes | 0 | 0 |

Decision: stay at 256 KiB. 1 MiB passes, but it increases max inflight
uncompressed bytes from 70 MiB to 110 MiB and does not materially improve any
operator-relevant metric. 128 KiB also passes, but there is no evidence here that
lowering the target improves drain or latency enough to justify extra backend
request fragmentation. 256 KiB remains the best supported default.

## Regression Harness

Implemented under `test/saw7500/` and `.github/workflows/saw7500-hardening.yml`.

Coverage:

- high-compression `repeated` and low-compression `random` payload profiles.
- exact uncompressed body size through `--payload-size-bytes`.
- request-size sweep wrapper for 128 KiB, 256 KiB, 512 KiB, and 1 MiB targets.
- analyzer gates for liveness kills/restarts, queue drain, refusals/rejections,
  generated-vs-delivered, backend rollout completion, settled p95/p99, and
  settled over-2s count.
- manual and nightly GitHub Actions workflow guarded to
  `Sawmills/opentelemetry-collector-contrib`.
- nightly default payload bodies: 256 KiB and 1 MiB.
- failure-only Slack notification to `C0B34NH76AY`, with `SLACK_BOT_TOKEN`
  scoped only to the notification step.
- artifact upload on every workflow run.

## Config And Observability Follow-Ups

LaunchDarkly flag `collectorsServiceCollectorLbQueueConfig` was read back at
version 108 with no variations containing `central_queue.lane_count`. Defaults
remain `onVariation=0`, `offVariation=1`.

Infra observability changes merged in `Sawmills/infra#759`.

Added alerts:

- `CollectorCentralQueueBackpressureHigh`: warning, non-page, sustained queue
  saturation, queue age over 120s, or backend p99 over 2s.
- `CollectorCentralQueueRejectedBytes`: critical/page, rejected compressed bytes
  rate greater than zero.

Validation:

- dashboard JSON parses with `python3 -m json.tool`.
- vmalert values parse with Ruby YAML.
- the four added alert rules parse with `promtool check rules`.
- `trunk check --fix` in the infra worktree reports no new issues.

## Probe Decision

The evidence does not justify changing probe semantics as part of the primary
fix. The bug path was queue drain/backpressure under backend timeout/churn; the
fixed collector removes that limiting factor in the local proof.

Defer `/live` and `/ready` split until one of these evidence triggers appears:

- green harness liveness kills or restarts while generated equals delivered and
  queue age is draining.
- green harness queue pressure can only recover by loosening liveness.
- readiness would shed load cleanly but the current shared `/healthcheck`
  endpoint causes kubelet to kill an otherwise responsive process.
- staging/non-BigID validation shows readiness flapping or liveness kills under
  normal backend churn after the queue/backpressure fix is present.
