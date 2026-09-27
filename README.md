# OpenTelemetry Collector Contrib — Observability Contributions

Three production-style improvements to the OpenTelemetry Collector Contrib repository, implemented as a junior Go/DevOps/Observability engineer.

## Contributions

### 1. Docker Stats Semantic Convention Migration
**Problem**: Docker Stats receiver used deprecated container semantic conventions (v1.21.0).  
**Solution**: Feature-gated migration to v1.42.0 conventions using two gates (`EmitV1ContainerConventions`, `DontEmitV0ContainerConventions`).  
**Impact**: ~3% overhead, full semantic compliance, zero breaking changes.

### 2. Transform Processor Tracing
**Problem**: No visibility into OTTL statement execution for debugging.  
**Solution**: Opt-in tracing via `processor.transform.emitOttlSpans` feature gate + `emit_trace_spans` config. Spans emitted for all 4 signal types (traces, metrics, logs, profiles).  
**Impact**: Zero overhead when disabled, ~55% CPU overhead when enabled (debugging only).

### 3. Resilient Error-Mode Defaults
**Problem**: `error_mode: propagate` drops valid telemetry on OTTL errors.  
**Solution**: Feature-gated default change to `ignore` for `processor/tailsampling` and `connector/signaltometrics`. Explicit config preserved.  
**Impact**: 43% more traces preserved (tailsampling), 25% more metrics (signaltometrics).

## Technical Highlights
- **Backward compatibility**: All changes opt-in via feature gates
- **Zero default overhead**: Feature gate checks at startup only
- **Comprehensive testing**: All existing tests pass, 50+ test calls updated
- **Honest benchmarking**: Measured actual overhead, documented limitations

## Repository Structure
```
docs/
├── current-state.md          # Analysis of upstream state
├── decisions.md              # Design decision records
├── benchmarks/               # Performance measurements
├── ai-assisted-development.md # AI usage transparency
├── interview-notes.md        # Technical interview prep
└── implementation-summary.md # This summary
```

## Skills Demonstrated
- Go, OpenTelemetry, OTTL, semantic conventions
- Feature gate patterns, backward compatibility
- Production observability, error handling
- Large codebase navigation, AI-assisted development
