# srclos-net collector roadmap

This fork extends opentelemetry-collector-contrib with the components we need to
migrate off Telegraf-style transmitters and to make the whole observability
pipeline *adaptive*. Everything here is built to stay upstream-shapable:
public API only, `mdatagen` discipline, and the semantics written down before
the code.

Design principles that apply to every component below:

1. **Fail toward visibility.** Any shared-state failure, rule error, or unknown
   condition behaves as "calm state" — data flows, never silently suppressed.
   A bug in this system must never create hidden silence during an incident.
2. **The exemplar rule.** Datapoints carrying exemplars must never be
   suppressed or dropped. Exemplars are the metric→trace link; they are
   exactly what anomaly windows exist to preserve.
3. **Semantics before syntax.** Each component's README states what a consumer
   can rely on (gaps vs tagged repeats, window behavior, restart behavior)
   before its config surface grows.
4. **Composition over components.** If a capability falls out of composing
   existing receivers/processors/connectors, it is config, not code. Only
   streaming/long-lived connections, SDK-grade state, or clean semantic
   shapes justify new components.

## Phase DB — `processor/deadband`

Stateful suppression of unchanged metric datapoints ("only send what changed")
with repeat-tagged heartbeats. Design doc: `processor/deadband/DESIGN.md`.

- **DB v1**: `absolute` + `percent` modes, gauge datapoints only, heartbeat by
  default (tagged `telemetry.deadband.repeated`), staleness force-send,
  shutdown flush, LRU state with cardinality cap, self-telemetry
  (`deadband.suppressed`, `deadband.heartbeat`, `deadband.series`).
  Config reserves `adaptive:` and `boost:` blocks, inert until X1/X2.
- **DB v2**: `attribute-set` mode, `group` (vector) mode, optional hysteresis,
  pass-through rules for cumulative/delta (zero-delta suppression), optional
  file-backed state via `filestorage`.
- Non-goals (explicit): deadbanding histograms; deadbanding traces.

## Phase X — cross-signal nervous system

- **X1**: `extension/signalstate` (shared named-window state store) +
  `processor/eventdetector` (OTTL rule evaluation per datapoint/log record,
  opens/closes named anomaly windows with hysteresis + cooldown, emits its own
  window metrics). Valuable standalone: the system becomes self-demonstrating.
- **X2**: `deadband` reads `signalstate`: boost-window flush of all currently
  suppressed series + suppression suspended for window + cooldown.
- **LOG v1**: `processor/logdedup` collapses repeated identical records
  (first + count attributes), pass-through during boost windows.
- **TR v1**: custom `tail_sampling` policy reading `signalstate` — higher
  keep-rate / keep-all-error-spans during windows.

Control-plane shape the phases converge to:

```
calm:    metrics deadbanded · logs dedup'd · probabilistic traces
boost:   deadband flushed+suspended · dedup open · traces kept
```

## Phases T1/T2 — Telegraf migration levers

- **T1 (`receiver/httpjson` — DONE, v1)**: poll JSON APIs → log records.
  Array-path resolution, body-key promotion, bearer/basic auth, timestamp
  layout. Planned v2: retries/backoff on 5xx, pagination, JMESPath, metric
  emission mode.
- **T1b (`receiver/execc` — DONE, v1)**: scheduled command → log records.
  v2: Stanza operator integration (grok/regex/jsonpath on command output),
  `stream` mode (long-running process, restart-on-exit), stderr handling.
- **T2 (`connector/textmetrics`)**: regex/grok over log attributes → metric
  datapoints. Composes with `filelog`, `tcplog`, `execc`, `httpjson` so that
  text-shaped telemetry has a metrics pathway. The current `countconnector`
  counts records; it cannot extract values.
- **T3**: per-Telegraf-input mapping table (config-only migration notes),
  executed as shadowed dual-writes (Telegraf + collector → same backend,
  diff per series, then cutover).

## Component inventory

| Component | Status | Phase |
|---|---|---|
| `receiver/execc` | v1 working (smoke-tested) | T1b |
| `receiver/httpjson` | v1 working (smoke-tested) | T1 |
| `processor/deadband` | v1 working (tested) | DB v1 |
| `extension/signalstate` | v1 working (tested) | X1 |
| `processor/eventdetector` | v1 working (tested) | X1 |
| `processor/logdedup` | design phase | LOG v1 |
| tail-sampling policy | design phase | TR v1 |
| `connector/textmetrics` | design phase | T2 |

X1 and X2 delivered 2026-11-20: `signalstate` (shared window store) +
`eventdetector` (OTTL rules on logs + metric datapoints opening/refreshing
windows with anti-flap cooldown) and `deadband` adaptive wiring (boost-entry
flash of tracked series, suppression suspended while the window is open, state
kept current so boost exit is burst-free). Remaining: log `dedup` and the
tail-sampling policy.

## Phase F — agent-operated, OpAMP-driven collection (parked; approved direction, not started)

Collection should become *changeable at all*: configs flow through git + review
+ OpAMP in minutes, shadowed experiments on every birth of a pipeline, so any
intelligence (human, HAL/Aiden, or fleet) can ride it:

- control plane: OpAMP (supervisor is already in cmd/ of this repo; server from
  opamp-go's example implementation) — AI authors config *PRs*, never commands
- policy layers by blast radius: introspection auto-approved; new low-volume
  pipelines auto + shadow dual-write; production exporter / PII-adjacent /
  fleet-wide changes require human approval
- schema invention: AI designs event schemas + Stanza mappings (our T-levers),
  registered against a future schema-registry extension; data model stays OTel
  (profiles being the ecosystem's own recently-added signal)
- fleet-level signalstate windows as policy (incident-wide boost)
- hard rules: telemetry content is untrusted data (prompt-injection via log
  lines is the primary threat); LLM in the authoring seat only, never the
  dataplane; volume budgets enforced in the config format; change-rate limits
  + cooldowns to prevent feedback oscillation; fleet kill switch to a safe
  baseline config
- design discussion: 2026-11-20 session (same day as the adaptive loop demo)
