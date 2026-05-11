# SAW-7500 Hardening Follow-Ups Plan

## Scope

This plan covers the six follow-ups requested after the SAW-7500 fix was proven and deployed:

1. Large highly-compressible payload stress.
2. Request-size sweep across 128 KiB, 256 KiB, 512 KiB, and 1 MiB.
3. Nightly regression harness with failed-result Slack notification.
4. Remove stale `lane_count` from LaunchDarkly variations.
5. Dashboard and alerting on the real limiting factors.
6. Liveness/readiness semantics hardening.

The plan is intentionally evidence-gated. No production-impacting config, LaunchDarkly, alert, or probe change should be applied until the harness produces a red/green result for the queue/backpressure fix path.

## Constraints

- Do not deploy to BigID as part of this work.
- Keep the proven SAW-7500 fix shape intact: `central_queue.num_consumers` is the operator-facing send-parallelism knob, lanes are derived/internal, and `target_compressed_bytes` remains 256 KiB unless evidence proves a better value.
- Preserve the existing mt-4-shaped local harness as the source of red/green proof.
- The harness must prove the queue/backpressure fix path before we decide on probe semantics.
- Any production-impacting change must have a red/green or dry-run proof before rollout.
- For this repo, run loadbalancingexporter Go tests from `exporter/loadbalancingexporter`, not from repo root.

## Deliverables

### 1. Large Compressible Payload Stress

Goal: isolate the remaining hypothesis that very large messages that compress well can strain memory enough to starve liveness.

Implementation direction:

- Extend `test/saw7500/run.sh` and templates to generate predictable high-compression log bodies.
- Add parameters for uncompressed record/body size and compressibility profile:
  - `--payload-profile repeated`
  - `--payload-profile random`
  - `--payload-size-bytes <n>`
- Keep the current mt-4 shape: 2 LBs, 4 workers, backend churn, constrained LB CPU/memory.
- Test at least:
  - 256 KiB uncompressed payloads.
  - 1 MiB uncompressed payloads.
  - Optional 4 MiB only if local resource use stays practical.
- Measure:
  - RSS, heap/sys, inflight uncompressed bytes.
  - liveness response status/latency and kills/restarts.
  - queue bytes/items/oldest age.
  - refused/rejected records/bytes.
  - backend p95/p99 and over-2s count.
  - generated vs delivered.

Acceptance:

- Red old collector must show at least one incident-shaped failure or memory/liveness stress signature.
- Green fixed collector must keep liveness stable, stay within inflight bounds, avoid sustained refused/rejected records, and keep backend latency below gate after settle.
- If 1 MiB fails green, keep 256 KiB and document the failure as evidence.

### 2. Request-Size Sweep

Goal: answer 256 KiB vs 1 MiB with evidence instead of intuition.

Implementation direction:

- Add a sweep wrapper around `test/saw7500/run.sh`.
- Run the same workload with:
  - `target_compressed_bytes=131072`
  - `target_compressed_bytes=262144`
  - `target_compressed_bytes=524288`
  - `target_compressed_bytes=1048576`
- Keep all other knobs fixed for comparability.
- Summarize each run into a compact table:
  - p95, p99, over-2s count.
  - queue bytes/items/oldest age.
  - inflight bytes and RSS.
  - refused/rejected.
  - generated=delivered.
  - liveness kills/restarts.

Acceptance:

- 256 KiB remains default unless another size improves latency/throughput without increasing memory, queue age, refusals/rejections, or liveness risk.
- 1 MiB is rejected unless it passes every gate and materially improves an operator-relevant metric.

### 3. Nightly Regression Harness With Slack Failure Reporting

Goal: prevent regressions in the incident shape without requiring manual reruns.

Recommended implementation:

- Add a GitHub Actions workflow in `opentelemetry-collector-contrib`.
- Schedule nightly plus manual dispatch.
- Build or pull required images:
  - fake backend image.
  - telemetrygen image.
  - fixed collector image or current branch image, depending on mode.
- Run a practical nightly profile:
  - shorter than the full incident reproduction.
  - still includes backend churn, slow backend injection, bounded queue, and liveness gate.
- Gate against regressions in:
  - liveness kills/restarts.
  - backend timeout pinning or p95/p99 near timeout.
  - queue oldest age not draining after backend recovery.
  - refused/rejected records or bytes.
  - generated vs delivered mismatch.
- Upload artifacts on every run:
  - summaries.
  - rendered config.
  - events.
  - pod restart state.
  - metrics snapshots.
- Notify Slack only on failure or cancellation.

Slack failure notification should include:

- Workflow name and run URL.
- Branch/commit.
- Red/green phase that failed.
- Top failing gates.
- Artifact URL.
- Suggested first command to reproduce locally.

Recommended Slack behavior:

- Failure only, no success spam.
- Post to one operational channel, not a DM.
- Use a repository secret for the webhook or Slack bot token.

Acceptance:

- Manual dispatch can run the harness and upload artifacts.
- A forced failing run posts to Slack with the artifact link.
- A passing run uploads artifacts and does not post to Slack.

### 4. Remove Stale `lane_count` From LaunchDarkly Variations

Goal: remove operator confusion. Runtime already ignores stale `lane_count`, but the variation still makes the config look two-knob.

Implementation direction:

- Identify all LaunchDarkly variations under the LB queue config flag that include `central_queue.lane_count`.
- Remove `lane_count` from recommended/default variations.
- Keep backward compatibility in collectors-service:
  - parse `lane_count` if present.
  - ignore it in normal generated config.
  - keep tests that prove stale `lane_count` is ignored.
- Dry-run read back the variation JSON before and after.

Acceptance:

- Recommended variations contain `num_consumers` but no `lane_count`.
- Live generated collector config still has effective derived lanes.
- No customer targeting changes are made while cleaning the variation payload.

### 5. Dashboard And Alerts On Limiting Factors

Goal: alert on queue/backpressure/backend latency before kubelet liveness becomes the symptom.

Recommended metrics:

- Queue pressure:
  - `otelcol_loadbalancer_central_queue_compressed_bytes`
  - `otelcol_loadbalancer_central_queue_saturation`
  - `otelcol_loadbalancer_central_queue_items`
  - `otelcol_loadbalancer_central_queue_oldest_item_age`
- Drops/refusals:
  - `otelcol_loadbalancer_central_queue_rejected_compressed_bytes`
  - `otelcol_receiver_refused_log_records_total`
- Dispatch:
  - `otelcol_loadbalancer_central_queue_active_consumers`
  - `otelcol_loadbalancer_central_queue_configured_consumers`
  - `otelcol_loadbalancer_central_queue_lanes`
- Backend health:
  - backend latency histogram p95/p99.
  - backend outcome failures.
  - backend reroute/quarantine/unquarantine/fail-open counters.
- Runtime/pod symptom checks:
  - pod restarts by container.
  - liveness probe failures.
  - LB replica count.
  - LB HPA current vs desired replicas.
  - RSS/heap/sys memory.

Recommended alert philosophy:

- Page on sustained data-loss risk: refusals/rejections, generated vs received mismatch where available, or queue age past a hard threshold.
- Ticket/Slack on early backpressure: queue age/bytes rising, backend p99 near timeout, active consumers saturated.
- Avoid paging purely on LB replica count; replica growth is a symptom unless paired with queue/latency/drop evidence.

Acceptance:

- Dashboard shows the whole causal chain in one place.
- Alerts distinguish early warning from page-worthy impact.
- Alert text points operators to the SAW-7500 runbook or ticket.

### 6. Liveness/Readiness Semantics Hardening

Goal: use the harness evidence to decide whether probe semantics are part of the fix path, then implement the split if the evidence shows kubelet liveness is still a symptom of controlled queue/backpressure.

Recommended implementation:

- First prove the queue/backpressure behavior in the local harness:
  - queue age/bytes are bounded or drain after backend recovery.
  - generated records equal delivered records.
  - no sustained refusals/rejections.
  - liveness failures do not occur while the process is otherwise draining.
  - p95/p99 backend latency stays below the agreed gate after settle.
- Decision gate:
  - If green harness results show stable liveness and clean drain, document that `/live`/`/ready` split is defense-in-depth and defer implementation.
  - If green harness results still show liveness risk during controlled backpressure/drain, implement the split as part of this workstream.
- Keep liveness as the narrow deadlock/process-stuck check.
- Use readiness to shed traffic under pressure.
- Evaluate whether `/healthcheck` should remain both readiness and liveness, or whether LB should expose separate endpoints:
  - `/live`: process event loop responds, collector not shutting down.
  - `/ready`: receiver/exporter pipeline can accept work, queue below pressure thresholds, HAProxy/backend readiness is usable.
- Do not loosen liveness as the primary fix. Probe changes are defense-in-depth.

Acceptance:

- Evidence report states the probe decision with metrics, not opinion.
- If the split is deferred:
  - evidence shows stable liveness, bounded/draining queue, no delivery mismatch, no sustained refusals/rejections, and latency below gate after settle.
  - the report documents the future trigger that would reopen `/live` and `/ready` implementation.
- If the split is implemented:
  - a synthetic queue/backpressure test can make readiness fail without causing liveness kill.
  - a true deadlock/stuck process still fails liveness.
  - staging rollout proves no readiness flapping under normal load.

## Recommended Sequencing

1. Add large-compressible payload support and local red/green tests.
2. Add request-size sweep wrapper and analyzer table.
3. Convert the harness into a manual GitHub Actions workflow.
4. Add nightly schedule and Slack-on-failure.
5. Clean stale `lane_count` from LaunchDarkly variations after tests prove no behavior change.
6. Add dashboard/alerts.
7. Run the evidence gate for probe semantics using the queue/backpressure harness output.
8. If evidence says probe split is needed, implement `/live` and `/ready`; otherwise document the deferral and required future trigger.

## Interview Decisions Needed

1. Slack failure destination:
   - Recommended: one existing ops/collector channel, failure-only.
   - Decision: send nightly harness failures to `#nightly`, channel ID `C0B34NH76AY`.
   - Discovery: `infra` has `ATMOS_APPLY_SLACK_WEBHOOK` and incoming-webhook workflow patterns, but no reusable GitHub Actions Slack bot token was found in `infra` or this repo. The default ArgoCD Slack SSM path from the Terraform module is not populated in plat prod/staging.
   - Owner decision: reuse the ArgoCD notifications Slack token for this workflow despite the security-coupling risk, because it avoids creating another token for this workstream.
   - Implementation note: mirrored the prod ArgoCD Kubernetes secret `argocd/argocd-notifications-secret` key `slack-token` into the repo Actions secret `SLACK_BOT_TOKEN` for `Sawmills/opentelemetry-collector-contrib`.
   - Workflow constraint: expose `SLACK_BOT_TOKEN` only as a single-step environment variable in the Slack failure-notification step; do not print it, persist it to artifacts, pass it to test containers, or expose it to pull-request code paths.
   - Use `chat.postMessage` to post to channel ID `C0B34NH76AY`.

2. Nightly runtime budget:
   - Recommended: 20-30 minutes max.
   - Decision: target 20-30 minutes max.
   - Alternative: full incident-scale nightly, more expensive and noisier.

3. Nightly branch policy:
   - Recommended: run on `main` nightly and on manual dispatch for branches.
   - Decision: run automatically on `main` nightly; support `workflow_dispatch` for branch validation.
   - Alternative: run on every PR touching `exporter/loadbalancingexporter`, more expensive.

4. Payload stress bounds:
   - Recommended: 256 KiB and 1 MiB required; 4 MiB optional/manual.
   - Decision: 256 KiB and 1 MiB are required nightly cases; 4 MiB is manual-only.

5. Alert destination and severity:
   - Recommended: Slack/ticket for early backpressure; page only for sustained refusals/rejections or liveness restarts.
   - Decision: Slack or ticket for early backpressure; page only for sustained refusals/rejections, liveness restarts, or delivery failure.

6. Liveness/readiness scope:
   - Recommended: decide from harness evidence, not intuition.
   - Decision: make the queue/backpressure proof part of this workstream. After the harness runs, decide whether `/live` and `/ready` split is needed; if evidence says yes, implement it in the same workstream. If evidence says no, document the deferral and future trigger.
   - Alternative: loosen current liveness thresholds now as a quick mitigation.

## First Implementation Plan After Approval

Once decisions are answered, start with a narrow PR in `opentelemetry-collector-contrib`:

1. Extend the fake backend or load generator path to support high-compression payload profiles.
2. Extend `test/saw7500/run.sh` with payload profile and request-size sweep modes.
3. Extend `test/saw7500/analyze.py` to emit the sweep comparison table and memory/liveness gates.
4. Add README examples for:
   - large-compressible red/green.
   - request-size sweep.
   - manual nightly-equivalent run.
5. Verify from `exporter/loadbalancingexporter`:
   - targeted Go tests.
   - harness red/green smoke.
   - analyzer summary.

Only after that should we add GitHub Actions, Slack notification, LaunchDarkly cleanup, dashboard/alerts, and the evidence-gated probe decision. If the evidence says probes are part of the remaining limiting factor, implement `/live` and `/ready` in this workstream.
