# SAW-7500 Local Red/Green Proof Harness

This harness is a local `kind` lab for reproducing SAW-7500-style LB cascade behavior:

- 2 LB pods.
- 4 backend pods behind a headless service.
- steady exact-byte OTLP log load through the harness load generator.
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

Build the local harness image. The same image runs the fake backend, tally
server, and exact-byte load generator:

```sh
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
  --target-compressed-bytes 262144 \
  --payload-profile repeated \
  --payload-size-bytes 262144
```

Artifacts are written under `artifacts/saw-7500/<timestamp>/`.

Payload profiles:

- `--payload-profile repeated` generates highly-compressible log bodies.
- `--payload-profile random` generates deterministic low-compression log bodies.
- `--payload-size-bytes` controls the exact uncompressed body size. Use
  `262144` for 256 KiB, `1048576` for 1 MiB, and reserve 4 MiB for manual
  stress runs.

Request-size sweep:

```sh
test/saw7500/sweep.sh -- \
  --green-image otelcontribcol-dev:saw-7500 \
  --payload-profile repeated \
  --payload-size-bytes 1048576
```

The sweep runs `target_compressed_bytes` values `131072`, `262144`, `524288`,
and `1048576`, then writes `sweep.md` and `sweep.json` under the sweep artifact
root.

## GitHub Actions

`.github/workflows/saw7500-hardening.yml` runs the same kind harness on manual
dispatch and nightly on `main` for `Sawmills/opentelemetry-collector-contrib`.
The default nightly profile runs highly-compressible `262144` and `1048576`
byte payloads with `target_compressed_bytes=262144`. It uploads artifacts for
all attempted payload sizes and posts to `#nightly` only on failure or
cancellation.

## Verdict

The analyzer requires:

- red has at least one incident signature: backend p99 pinned at timeout,
  queue over budget, rejected/refused records, delivery mismatch, or an LB
  liveness restart.
- green has zero LB restarts.
- green queue bytes stay below capacity.
- green oldest queue age returns near baseline.
- green refused/rejected deltas are zero.
- green backend p95 is under 2s after settle, settled over-2s count is zero
  by default, and p99 is not pinned at 5s.

Use `--green-max-over-2s-count <n>` only for intentionally noisy/manual
profiles. The nightly-equivalent proof should keep the default zero-over-2s
settled gate.

The local default does not require kubelet to kill the LB. In kind, the
deterministic proof target is the limiting factor that caused the cascade:
timeout-pinned backend exports plus queue growth. Use
`--require-red-liveness-restart` only when the local resource limits or probe
sensitivity are tight enough to reproduce the downstream kubelet kill too. The
probe can be tightened with `--liveness-timeout-seconds` and
`--liveness-failure-threshold`; use the same values for red and green.

If red does not fail any incident predicate, the load/rollout simulation is too weak.
Use `--strict-red` when you specifically need the full historical incident shape
instead of the default "any incident-shaped signature" gate.
