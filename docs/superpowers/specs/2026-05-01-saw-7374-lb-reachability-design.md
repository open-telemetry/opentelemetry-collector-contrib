# SAW-7374 Collector LB Reachability Design

## Goal

Make collector load-balancer routing degrade safely when Kubernetes says backend pods are ready but an individual LB pod cannot reach some of those backend endpoints.

The work is evidence-gated:

1. Prove the real BigID staging configuration that was running during the May 1 failure.
2. Reproduce the partial-reachability behavior in an isolated Sawmills staging canary.
3. Implement active local probing only if existing reactive `endpoint_health` is insufficient.

BigID staging is read-only for this task. Do not change its config, deploy into it, or inject failures there. Production is out of scope.

## Current Behavior

`loadbalancingexporter` already has `endpoint_health`.

When enabled, it:

- records resolver-present endpoints as present,
- quarantines endpoints after endpoint-local export failures,
- rebuilds the eligible routing ring without quarantined endpoints,
- reroutes failed telemetry once when `reroute_on_failure` is enabled,
- fails open if every resolver-present endpoint is quarantined,
- emits backend state, quarantine, reroute, and fail-open metrics.

`collectors-service` already renders the existing `endpoint_health` config through the LaunchDarkly-backed LB queue config path. `sawmills-collector` already has `forwarding_health.backend_usability` so LB readiness can fail when eligible backends fall below a configured threshold or fail-open/reroute failures grow.

Do not add a separate `local_backend_health` surface. Any new behavior should extend `endpoint_health`.

## Gate 1: BigID Staging Evidence

Collect read-only evidence from the real BigID `staging-us-east-1` deployment/revision implicated by SAW-7374.

Capture:

- collector image/version,
- Helm chart and revision,
- actual generated LB collector config,
- every rendered `endpoint_health` field,
- `otlp_timeout`,
- resolver settings, including DNS interval/timeout or any Kubernetes resolver behavior,
- `forwarding_health.backend_usability`,
- relevant LB logs and metrics around retries/rejects if still available.

Source of truth must be the generated live config and runtime state for the impacted deployment/revision, not only LaunchDarkly defaults.

Gate 1 outcomes:

- If `endpoint_health` or backend-usability settings were disabled, missing, version-gated out, or misrendered, stop implementation and document a rollout/config fix.
- If config was present and sane, continue to Gate 2.

## Gate 2: Isolated Staging Canary

Create an isolated Sawmills staging canary namespace/deployment that mimics the BigID failure mode without touching BigID.

Canary shape:

- LB collector pods replaying the Gate 1 generated BigID LB config/version as the canary baseline.
- Preserve the generated exporter shape from Gate 1, including `protocol.otlp`
  timeout, `retry_on_failure`, `sending_queue`, log/metric batcher settings,
  routing settings, resolver settings, `endpoint_health`, and
  `forwarding_health.backend_usability`.
- Change only isolated-resource details: namespace/resource names, backend endpoint targets, fault-injection targets, credentials/secrets needed for canary traffic, and labels/metadata.
- Multiple backend endpoints behind the same resolver type and timing settings used by the BigID LB exporter.
- A controlled subset of backend endpoints that times out from the LB pod while at least one other backend remains reachable.
- Traffic directed only at the canary LB.

Validate current reactive `endpoint_health` before writing active-probe code.

Pass/fail evidence must include:

- A config diff/checksum showing the canary replayed the Gate 1 LB config with only the expected isolated-resource deltas.
- `otelcol_loadbalancer_backend_quarantine_total` increments for the known-bad endpoint.
- `otelcol_loadbalancer_backend_state{state="quarantined"}` is emitted for the bad endpoint.
- Eligible backend count drops after quarantine trips / after the first endpoint-local failure.
- `otelcol_loadbalancer_backend_reroute_total{result="success"}` increases while a reachable backend exists.
- `otelcol_loadbalancer_backend_reroute_total{result="failure"}` does not keep growing while a reachable backend exists.
- The persistent partition remains in place for at least two
  `endpoint_health.quarantine_duration` expiry cycles, plus resolver poll and
  `forwarding_health.backend_usability` poll intervals.
- Across those captured expiry cycles, there are no recurring reject logs or
  receiver refusals when the exporter re-admits the same locally unreachable
  endpoint.
- `otelcol_loadbalancer_backend_fail_open_total` increments only when every candidate backend is bad or no eligible backend remains.

Gate 2 outcomes:

- If reactive quarantine is sufficient, close SAW-7374 with evidence and no exporter code.
- If reactive quarantine is not sufficient because customer data must fail first and causes rejects/refusals before quarantine stabilizes routing, implement active local probing.
- If persistent partial partition rejects/refusals recur after quarantine expiry
  re-admits the bad endpoint, reactive quarantine is insufficient and active
  local probing is required.
- If the canary cannot replay the Gate 1 LB config/version with only isolated-resource deltas, Gate 2 is inconclusive and cannot close SAW-7374 without either replay parity or a documented config/rollout fix.

Clean up the canary namespace and any temporary resources after the test.

## Active Probe Design If Needed

Extend `endpoint_health` with optional active local probing.

Config shape:

```yaml
endpoint_health:
  enabled: true
  reroute_on_failure: true
  active_probe:
    enabled: true
    type: tcp_connect
    interval: 5s
    timeout: 250ms
    jitter: 20%
    max_concurrency: 4
    fall: 2
    rise: 2
```

Default probe semantics:

- Use `tcp_connect` to the exact backend export endpoint and port used by the loadbalancing exporter.
- Do not use gRPC health checks by default.
- Do not send telemetry writes as probes.
- Keep the export path fail-open by default.

The active probe state should feed the existing endpoint-health eligibility model:

- A backend becomes locally unhealthy after `fall` failed probes.
- A backend becomes locally healthy again after `rise` successful probes.
- Healthy/unhealthy probe state must update the same eligible routing ring used by reactive quarantine.
- Probe unhealthy/healthy transitions must update the existing
  `otelcol_loadbalancer_backend_state` series so
  `forwarding_health.backend_usability` sees the same eligible/quarantined
  counts that routing uses.
- Probe-driven exclusion and recovery must also use the existing
  quarantine/unquarantine/fail-open signal family, with reason labels if needed,
  rather than only emitting probe-specific metrics.
- If zero locally healthy backends remain, endpoint health should preserve fail-open behavior rather than blackholing telemetry.
- `forwarding_health.backend_usability` remains the mechanism that drains a bad LB pod when locally eligible backends are below threshold.

Config validation:

- `type` must be a known probe type. Initially, only `tcp_connect` is valid.
- `interval` and `timeout` must be positive durations.
- `timeout` must be shorter than `interval`.
- `jitter` must parse as a percentage from `0%` through `100%`.
- `max_concurrency` must be positive.
- `fall` and `rise` must be positive integers.
- Both exporter config validation and collectors-service LaunchDarkly/generator
  validation must reject invalid active-probe values before rendering a
  collector config.

Probe load controls:

- Add jitter so LB pods do not probe endpoints in lockstep.
- Enforce `max_concurrency`.
- Keep default interval conservative.
- Allow faster intervals only by explicit per-customer/canary config.

Metrics:

- Probe state changes must feed existing backend state/quarantine/unquarantine
  and fail-open metrics used by routing and `forwarding_health`.
- Add probe-specific metrics only as supplemental debugging signals, such as
  probe failures and latency by low-cardinality reason/state.
- If implementation cannot reuse those existing metrics directly, update
  `forwarding_health.backend_usability` parsing in the same change so readiness
  still derives from the active-probe-aware eligible backend set.

## Code Boundaries If Active Probe Is Needed

Primary repo: `sawmills-collector-contrib`.

Likely exporter changes:

- Extend `EndpointHealthConfig` and config schema.
- Add validation/defaults for `active_probe`.
- Add an endpoint health probe loop owned by `loadBalancer` lifecycle.
- Feed probe state into the endpoint-health manager without duplicating routing state.
- Rebuild the eligible ring when probe state changes.
- Preserve resolver-change and exporter-stopping reroute behavior.
- Preserve current behavior when `active_probe.enabled` is false.

Config propagation repo: `collectors-service`.

Likely generator changes:

- Extend `LBEndpointHealthConfig` with `ActiveProbe`.
- Validate every active-probe field before rendering, including probe `type`, durations, jitter, concurrency, `fall`, and `rise`.
- Version-gate active-probe rendering to the first collector version that contains the exporter change.
- Add LaunchDarkly parsing/generator tests.

Runtime bundle repo: `sawmills-collector`.

Likely release changes:

- Bump the loadbalancingexporter fork after exporter merge.
- Update the minimum collector version constant in collectors-service after release.

## Testing

Use TDD for implementation.

Exporter tests should prove:

- active probe config loads and validates,
- invalid probe `type`, `interval`, `timeout`, `timeout >= interval`,
  `jitter`, `max_concurrency`, `fall`, and `rise` values fail exporter
  validation,
- unhealthy local endpoints are excluded from routing when healthy alternatives exist,
- after `rise` successful probes, a recovered endpoint is re-added to the eligible routing ring,
- probe state transitions update existing backend state/quarantine/unquarantine
  and fail-open signals used by forwarding-health readiness,
- normal consistent hashing is preserved when all endpoints are healthy,
- zero locally healthy endpoints fail open by default,
- resolver-change and exporter-stopping reroute behavior remains intact,
- probe loops stop cleanly on shutdown.

Integration-style test should simulate multiple resolver backends where one
endpoint times out from the LB pod. After active-probe `fall`, no customer
telemetry should be sent to the unhealthy endpoint while another healthy backend
exists. After the endpoint becomes reachable again and reaches `rise` successful
probes, the test must prove the endpoint is eligible again and the routing ring
is rebuilt so traffic can return to it.

Collectors-service tests should prove:

- LaunchDarkly JSON parses into the new active-probe config,
- rendered LB exporter config includes active-probe fields only for supported collector versions,
- invalid intervals, timeouts, jitter, concurrency, `type`, `fall`, and `rise` fail generation clearly,
- negative cases include at least `fall: 0`, `rise: -1`, and an unknown probe `type`,
- omitted active-probe config preserves current output.

## Operational Safety

- No production changes in this task.
- No BigID staging mutations.
- Canary fault injection only in isolated Sawmills staging resources.
- Feature disabled by default.
- Rollout controlled by collectors-service / LaunchDarkly.
- Document Gate 1 and Gate 2 evidence back on SAW-7374 before any code PR is considered complete.
