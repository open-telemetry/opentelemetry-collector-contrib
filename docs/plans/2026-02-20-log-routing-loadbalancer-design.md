# Log Routing by Resource Attributes in Loadbalancing Exporter

## Problem Statement

The loadbalancing exporter currently routes logs only by `traceID`, falling back to a random key when no trace ID is present. This means logs from the same application/service can scatter across multiple backends randomly, breaking stateful downstream features like log reduction, throttling, and deduplication.

Traces and metrics already support configurable routing keys (`service`, `resource`, `attributes`, etc.), but logs do not. The `routing_key` config field is silently ignored for logs.

## Goals

- Route all logs from the same application/service consistently to the same backend
- Support `service`, `resource`, and `attributes` routing keys for logs
- Enable users to specify any resource/log attribute(s) as the routing key (e.g., `service.name`, `job`, `k8s.namespace.name`)
- Default to `service` (service.name) routing for logs
- Remove the traceID-based log routing (not applicable for log use cases)
- Add benchmark tests to detect performance regressions

## Non-Goals

- Changing trace or metrics exporter routing behavior
- Adding new routing key types beyond what already exists in the codebase
- Implementing a log-specific merge utility beyond the simple `MoveAndAppendTo` pattern

## Design

### Architecture

```
ConsumeLogs(plog.Logs)
    |
    v
splitLogsByX(ld) -> map[string]plog.Logs   // split by resource, keyed by routing ID
    |
    v
for routingID, batch := range batches:
    exp, endpoint = lb.exporterAndEndpoint([]byte(routingID))
    merge into exporter buckets
    |
    v
for exp, logs := range logsByExporter:
    exp.ConsumeLogs(ctx, logs)
```

### Supported Routing Keys

| `routing_key` | Behavior | Key Source |
|---|---|---|
| `"service"` (default) | Route by `service.name` | Resource attribute |
| `"resource"` | Route by hash of all resource attributes | `identity.OfResource()` |
| `"attributes"` | Route by composite key from user-specified attributes | Resource attrs, scope attrs, log record attrs |

### Component Changes

#### 1. `logExporterImp` struct (`log_exporter.go`)

Add `routingKey routingKey` and `routingAttrs []string` fields.

#### 2. `newLogsExporter` constructor

Add routing key parsing with switch statement:
- `"service"` or `""` (default) -> `svcRouting`
- `"resource"` -> `resourceRouting`
- `"attributes"` -> `attrRouting` (reads `RoutingAttributes` from config)
- All other values -> error

#### 3. New split functions

**`splitLogsByServiceName(ld plog.Logs) (map[string]plog.Logs, []error)`**
- Iterates ResourceLogs, extracts `service.name` from resource attributes
- Returns error for resources missing `service.name` (consistent with metrics exporter)
- Groups scope logs under the same service.name into one plog.Logs

**`splitLogsByResourceID(ld plog.Logs) map[string]plog.Logs`**
- Uses `identity.OfResource(rm.Resource()).String()` as the routing key
- Groups by full resource identity hash

**`splitLogsByAttributes(ld plog.Logs, attrs []string) map[string]plog.Logs`**
- Builds composite key from specified attribute names
- Attribute lookup order: resource attributes -> scope attributes -> log record attributes
- Log-specific pseudo attributes: `log.severity`, `log.body`

#### 4. `ConsumeLogs` rewrite

Replace current batchpersignal.SplitLogs + per-record dispatch with metrics-style pattern:
- Switch on routingKey to select split function
- Hash each routing ID to get exporter via consistent hash ring
- Merge logs destined for the same exporter
- Dispatch to each exporter with telemetry recording

#### 5. `mergeLogs` helper (`helpers.go`)

```go
func mergeLogs(l1, l2 plog.Logs) plog.Logs {
    l2.ResourceLogs().MoveAndAppendTo(l1.ResourceLogs())
    return l1
}
```

#### 6. Dead code removal

- Remove `traceIDFromLogs()` function
- Remove `random()` function
- Remove `consumeLog()` method
- Remove `batchpersignal` import from log exporter
- Remove `pcommon` import (no longer needed for TraceID)

#### 7. README updates

- Update routing_key table to include logs column
- Remove disclaimer that routing_key has no effect on logs
- Add log-specific configuration examples
- Document `routing_attributes` usage with log attributes

### Test Plan

#### Unit Tests
- `TestSplitLogsByServiceName` — single/multiple resources, missing service.name
- `TestSplitLogsByResourceID` — single/multiple resources, identical/different resources
- `TestSplitLogsByAttributes` — single/multiple attributes, missing attributes, pseudo attributes
- `TestNewLogsExporterWithRoutingKey` — all valid keys, invalid key rejection
- `TestConsumeLogsWithServiceRouting` — end-to-end with service routing
- `TestConsumeLogsWithResourceRouting` — end-to-end with resource routing
- `TestConsumeLogsWithAttributeRouting` — end-to-end with attribute routing
- `TestConsumeLogsConsistentRouting` — verify same key always routes to same backend

#### Benchmark Tests
- `BenchmarkSplitLogsByServiceName` — performance baseline for service routing
- `BenchmarkSplitLogsByResourceID` — performance baseline for resource routing
- `BenchmarkSplitLogsByAttributes` — performance baseline for attribute routing
- `BenchmarkConsumeLogsServiceRouting` — end-to-end performance

### Breaking Changes

- Logs are no longer routed by traceID. The default changes to `service` (service.name).
- The `traceIDFromLogs()` and `random()` functions are removed.
- Existing configs that relied on traceID-based log routing will now route by service.name instead.

### Files Modified

| File | Change |
|---|---|
| `log_exporter.go` | Major rewrite: routing key support, split functions, ConsumeLogs rewrite |
| `log_exporter_test.go` | Major rewrite: new tests for all routing modes, benchmarks |
| `helpers.go` | Add `mergeLogs` function |
| `README.md` | Update routing_key docs, add log examples |
| `config.go` | No changes needed (routing_key and routing_attributes already exist) |
