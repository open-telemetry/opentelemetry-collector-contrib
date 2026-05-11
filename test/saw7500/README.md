# SAW-7500 Local Red/Green Proof Harness

This harness is a local `kind` lab for reproducing SAW-7500-style LB cascade behavior:

- 2 LB pods.
- 4 backend pods behind a headless service.
- steady OTLP log load through `telemetrygen`.
- backend rollout plus deterministic slow-backend behavior.
- automatic red/green evidence capture.

The fake backend also exposes `/drain` and `/undrain` on its HTTP port. Draining
makes readiness and gRPC health return not-serving while liveness stays healthy,
which lets the lab exercise backend removal without killing the backend process.

The expected proof shape is:

- red: old/prod-like LB behavior fails under the rollout/load shape.
- green: candidate fixed LB behavior passes the same load, pod count, and rollout cadence.

## Prerequisites

```sh
kind version
kubectl version --client=true --output=yaml
docker version
```

Build local images:

```sh
make docker-telemetrygen
docker build -t saw7500-fakebackend:latest test/saw7500/fakebackend
```

When testing a local collector build:

```sh
make docker-otelcontribcol
docker tag otelcontribcol otelcontribcol-dev:saw-7500
```

## Run

```sh
test/saw7500/run.sh \
  --red-image public.ecr.aws/s7a5m1b4/sawmills-collector:1.936.0 \
  --green-image otelcontribcol-dev:saw-7500 \
  --lb-replicas 2 \
  --workers 4 \
  --num-consumers 30 \
  --target-compressed-bytes 262144
```

Artifacts are written under `artifacts/saw-7500/<timestamp>/`.

## Verdict

The analyzer requires:

- red has at least one incident signature: backend p99 pinned at timeout,
  queue over budget, rejected/refused records, delivery mismatch, or an LB
  liveness restart.
- green has zero LB restarts.
- green queue bytes stay below capacity.
- green oldest queue age returns near baseline.
- green refused/rejected deltas are zero.
- green backend p95 is under 2s after settle and p99 is not pinned at 5s.

The local default does not require kubelet to kill the LB. In kind, the
deterministic proof target is the limiting factor that caused the cascade:
timeout-pinned backend exports plus queue growth. Use
`--require-red-liveness-restart` only when the local resource limits or probe
sensitivity are tight enough to reproduce the downstream kubelet kill too. The
probe can be tightened with `--liveness-timeout-seconds` and
`--liveness-failure-threshold`; use the same values for red and green.

If red does not fail any incident predicate, the load/rollout simulation is too weak.
