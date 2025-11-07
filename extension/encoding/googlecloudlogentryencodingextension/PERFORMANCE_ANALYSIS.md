# Performance Analysis: UnmarshalLogs Implementation

## Current Implementation Issues

### 1. Structure Overhead
The current `UnmarshalLogs` implementation creates a **new ResourceLogs and ScopeLogs for every single log line**:

```go
func (ex *ext) handleLogLine(logs plog.Logs, logLine []byte) error {
    // ...
    rl := logs.ResourceLogs().AppendEmpty()  // NEW ResourceLogs for each log
    r := rl.Resource()
    scopeLogs := rl.ScopeLogs().AppendEmpty()  // NEW ScopeLogs for each log
    // ...
}
```

**Performance Impact:**
- For N log lines, this creates N ResourceLogs and N ScopeLogs structures
- Each ResourceLogs contains:
  - Resource attributes map
  - Schema URL
  - ScopeLogs slice
- Each ScopeLogs contains:
  - InstrumentationScope (name, version, attributes)
  - Schema URL  
  - LogRecords slice

**Memory Overhead:** For 1000 log lines, this creates:
- 1000 ResourceLogs structures
- 1000 ScopeLogs structures
- Significant overhead in slice allocations and memory fragmentation

### 2. plog.Logs Internal Structure

From the generated code analysis:
- `plog.Logs` wraps `internal.ExportLogsServiceRequest`
- It contains a `ResourceLogsSlice` (slice of ResourceLogs)
- Each ResourceLogs contains:
  - Resource (attributes map)
  - ScopeLogsSlice (slice of ScopeLogs)
- Each ScopeLogs contains:
  - InstrumentationScope
  - LogRecordSlice (slice of LogRecords)

**Structure Hierarchy:**
```
plog.Logs
  └── ResourceLogsSlice[]
      └── ResourceLogs
          ├── Resource (attributes)
          └── ScopeLogsSlice[]
              └── ScopeLogs
                  ├── InstrumentationScope
                  └── LogRecordSlice[]
                      └── LogRecord
```

### 3. Interface Constraints

The `LogsUnmarshalerExtension` interface implements `plog.Unmarshaler`:
```go
type LogsUnmarshalerExtension interface {
    extension.Extension
    plog.Unmarshaler  // UnmarshalLogs(buf []byte) (plog.Logs, error)
}
```

**Key Constraint:** The interface **requires returning a complete `plog.Logs` batch**. Streaming individual logs is not possible within this interface design.

## Performance Optimization Opportunities

### Option 1: Reuse ResourceLogs/ScopeLogs (Recommended)

Instead of creating new structures for each log, reuse a single ResourceLogs and ScopeLogs, appending all log records to the same scope:

```go
func (ex *ext) UnmarshalLogs(buf []byte) (plog.Logs, error) {
    logs := plog.NewLogs()
    
    // Create ONE ResourceLogs and ONE ScopeLogs for all logs
    rl := logs.ResourceLogs().AppendEmpty()
    scopeLogs := rl.ScopeLogs().AppendEmpty()
    
    scanner := bufio.NewScanner(bytes.NewReader(buf))
    for scanner.Scan() {
        line := scanner.Bytes()
        if err := ex.handleLogLine(rl, scopeLogs, line); err != nil {
            return plog.Logs{}, err
        }
    }
    
    return logs, nil
}
```

**Benefits:**
- Reduces structure overhead from O(N) to O(1) for ResourceLogs/ScopeLogs
- Only LogRecords are allocated per log line
- Significantly reduces memory allocations
- Better memory locality (all log records in same slice)

**Limitations:**
- All logs share the same resource attributes (if logs have different resources, they would need separate ResourceLogs)
- All logs share the same scope (if logs have different scopes, they would need separate ScopeLogs)

### Option 2: Grouping by Resource/Scope (Advanced)

For logs with different resources or scopes, group them intelligently:

```go
// Group logs by resource attributes hash
resourceGroups := make(map[string]plog.ResourceLogs)
scopeGroups := make(map[string]plog.ScopeLogs)

// For each log, determine resource/scope key and append to appropriate group
```

**Trade-offs:**
- More complex implementation
- Requires hashing/comparison overhead
- Better semantic grouping
- May not be necessary if all logs share same resource/scope

### Option 3: Streaming Interface (Not Possible)

A streaming interface would require changing the OpenTelemetry Collector API:

```go
// NOT POSSIBLE with current interface
type StreamingLogsUnmarshaler interface {
    UnmarshalLogs(buf []byte, callback func(plog.LogRecord) error) error
}
```

This would require:
- Modifying the collector framework
- Changing all consumers
- Breaking backward compatibility

## Recommendation

**Implement Option 1** (Reuse ResourceLogs/ScopeLogs):
1. Most logs from the same source share the same resource/scope
2. Massive reduction in memory overhead
3. Simple implementation
4. No API changes required

If logs have different resources/scopes, we can enhance with Option 2 later.

## Expected Performance Improvements

For a buffer with 1000 log lines:

**Before:**
- 1000 ResourceLogs allocations
- 1000 ScopeLogs allocations  
- 1000 LogRecord allocations
- Total: ~3000 structure allocations

**After:**
- 1 ResourceLogs allocation
- 1 ScopeLogs allocation
- 1000 LogRecord allocations
- Total: ~1002 structure allocations

**Memory Reduction:** ~66% reduction in structure overhead
**Allocation Reduction:** ~67% reduction in allocations

