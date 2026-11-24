# processor/deadband — design (DB v1)

Status: design phase. Semantics are agreed here first; config is reserved so
it does not churn later.

## What a consumer can rely on

These are the load-bearing promises. Every implementation decision defers to
them.

1. **No silence without accounting.** A suppressed datapoint is either
   eventually restated (heartbeat, tagged `telemetry.deadband.repeated: true`
   with the last-sent value) or counted in self-telemetry. There is no
   "forgot to send" state.
2. **Staleness is bounded.** If a series has not been emitted for
   `stale_after`, the last-sent value is re-emitted regardless of change.
   Default staleness bound will exist and be documented; `stale_after = 0`
   (unbounded) is allowed but must be explicit in config.
3. **Shutdown flushes.** On collector shutdown, all series in state are
   emitted once with their last known values.
4. **Fail toward visibility.** Any state-store or evaluation error processes
   the datapoint as if calm-state (i.e., pass through unchanged). The boost
   path never suppresses on error.
5. **Exemplars are inviolable.** Datapoints carrying exemplars are never
   suppressed, deadbanded, or dropped.
6. **Scope is explicit.** v1 applies to *gauge* datapoints only. Histograms,
   Sum (cumulative) and Summary datapoints pass through untouched.
7. **Restart is a gap, not a state reset.** After restart the series
   re-appears as "new": first datapoint is always emitted. (Optional
   file-backed state lands in DB v2.)

## Reserved config surface (inert until wired)

```yaml
processors:
  deadband:
    # match: metric selection (match_type strict|regexp, name set)
    # mode: absolute | percent
    # threshold
    # stale_after: bound on silence before forced restatement
    # repeat_attribute: "telemetry.deadband.repeated"
    # state: { max_series: N }          # LRU cardinality cap
    # adaptive:                         # reserved (X2 wiring)
    #   enabled: false
    #   window_flush_on_boost: true
    # boost:                            # reserved (X2 wiring)
    #   attribute: "telemetry.deadband.boost"
```

## Mode semantics (DB v1)

### absolute

Emit iff `|v - last_sent_v| >= threshold`. Baseline = the last *emitted*
value, not the last observed value (dead-chains never drift).

### percent

Emit iff `|v - last_sent_v| / max(|last_sent_v|, min_reference) >= threshold`.
If `|last_sent_v| < min_reference` (default: the threshold itself), the rule
degrades to absolute mode with `threshold` — avoiding division noise near
zero. `min_reference` configurable.

### Heartbeats

On `stale_after` silence: emit `last_sent_v` again with
`repeat_attribute = true`. Heartbeats keep the *last-sent* value, never a
partially-observed newer one, so consumers can trust a repeated datapoint as
"nothing has changed enough to move the reading."

### Self-telemetry (mdatagen)

- `otelcol_deadband_suppressed_total{receiver=…}` — count by metric name
- `otelcol_deadband_heartbeat_total{receiver=…}`
- `otelcol_deadband_series` — current LRU occupancy vs cap

If these are missing or zero during high-volume gauge traffic, something is
wrong with the processor itself — they are the deadband's own deadband.

## Out of scope for DB v1 (recorded intent)

- `attribute-set` mode (DB v2)
- group/vector mode (DB v2)
- hysteresis (DB v2)
- cumulative/delta zero-suppression (DB v2)
- file-backed state via filestorage (DB v2)
- anything traces (phase TR)
