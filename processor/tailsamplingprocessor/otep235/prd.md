# OTEP 235 Tail Sampling Implementation

This document summarizes the OTEP 235 consistent probability sampling implementation for the tail sampling processor.

## Overview

The tail sampling processor has been updated to support OTEP 235 consistent probability sampling through:

1. **Threshold-based sampling decisions** using `pkg/sampling`
2. **TraceState propagation** with final sampling thresholds  
3. **Cache system** storing `sampling.Threshold` instead of boolean decisions
4. **Late-arriving span evaluation** against cached thresholds

## Key Changes

### Core OTEP 235 Features

- **TraceStateManager**: Parses and updates W3C TraceState with OTEP 235 fields
- **Threshold-based decisions**: Policies coordinate through `FinalThreshold` in TraceData
- **Randomness extraction**: From TraceState `rv` value or derived from TraceID
- **Consistent sampling**: Late-arriving spans evaluated against cached final thresholds

### Simplified Implementation

- **Trace-level threshold management** instead of complex per-span tracking
- **Essential functionality only** - removed complex telemetry tests and per-span calculations
- **Backward compatibility** - all existing functionality preserved

## Architecture

```text
Incoming Spans → TraceStateManager.InitializeTraceData() → Policy Evaluation → Final Threshold → Cache Decision → Late Span Evaluation
```

## Testing

Core functionality validated through:

- `TestOTEP235Integration` - Basic OTEP 235 TraceState handling
- `TestPhase5_TraceStateProcessing` - End-to-end trace processing
- Existing processor tests - Backward compatibility verification

## TODOs for Enhanced Consistency

1. **Randomness validation**: Check all spans in a trace have the same randomness value
2. **Upstream detection**: Log warnings for inconsistent sampling across spans
3. **Threshold validation**: Detect conflicting thresholds within traces

The implementation prioritizes essential OTEP 235 compliance while maintaining simplicity and performance.