# Updated tail sampling processor

This project covers updating the tail sampling processor in this
repository to use new conventions, based on the OpenTelemetry
consistent probability sampling specifications.

In the processor/tailsamplingprocessor/otep235 directory, find several
relevant documents and artifacts:

- tracestate-probability-sampling.md: contains new specification
- tracestate-handling.md: contains syntax details for W3C tracestate
- 0235-sampling-threshold-in-trace-state.md: text of OTEP 235 with original details
- 0250-Composite_Samplers.md: text of OTEP 250 with design for SDK samplers

Two sub-directories contain prototype implementations of the SDK
feature for composable, rule-based samplers in Golang (go-sampler) and
Rust (rust-sampler) based on OTEP 250.

The tail sampling processor in processor/tailsamplingprocessor is a
community-owned component with a configurable rule engine with
features allowing users to shape their trace data.  These rules
include rate-limiting and probability-based decisions, but they were
designed before OTEP 235 was published.

Note that there is a helper library for OTEP 235 in pkg/sampling
which has been incorporated into the intermediate span sampler
processor in processor/probabilisticsamplerprocessor.

## Current State Analysis

### Existing Implementation Limitations

The current tailsamplingprocessor implementation has several key limitations that prevent OTEP 235 compliance:

1. **Probabilistic Sampling**: Uses custom hash-based algorithm (`hashTraceID`) with configurable salt, incompatible with OTEP 235 threshold-based approach
2. **Rate Limiting**: Simple allow/deny mechanism using `rate.Limiter` without threshold adjustment or trace consistency
3. **TraceState Handling**: No systematic parsing or propagation of TraceState values; only basic key-value matching in `trace_state_filter`
4. **Policy Isolation**: Each policy operates independently without coordination or consistency guarantees
5. **Legacy Architecture**: Processor design predates OTEP 235 and lacks infrastructure for threshold management

### Available Infrastructure

Positive aspects that can be leveraged:

1. **pkg/sampling Integration**: Comprehensive OTEP 235 utilities already available:
   - `ProbabilityToThreshold()` and `ThresholdToProbability()` conversion functions
   - `OTelTraceState` for TraceState parsing and manipulation
   - `ShouldSample()` for consistent sampling decisions
   - `RandomnessFromTraceID()` for extracting randomness values

2. **Processor Architecture**: Well-designed batch processing and policy framework:
   - Policy evaluator interface suitable for extension
   - Trace accumulation and batch flushing infrastructure
   - Caching system for performance optimization
   - Comprehensive telemetry and metrics reporting

3. **Testing Infrastructure**: Robust testing framework including unit tests, benchmarks, and fuzz testing

## Definition of done

After this project is complete, the tailsampling processor will be
fully compatible with OTEP 235, meaning that changes of effective
probability for a trace correctly changes the sampling threshold in
individual span tracestate values.

## New tailsamplingprocessor requirements

The OpenTelemetry tracestate threshold value known as T (encoded `th`)
directly indicates effective sampling probability. Changes in
effective probability must be achieved in a way consistent with this
definition.

OTEP 250 demonstrates how to implement a rule-based head sampler which
maintains consistent probability sampling. Many of the aspects of the
updated processor will be similar to the SDK samplers, however
establishing rate limits is different for tail sampling compared with
head sampling, in part because spans arrive in batches, and largely
because the unit of sampling is whole traces.

### Core Requirements

1. **TraceState Threshold Handling**:
   - Parse incoming TraceState for `th` (threshold) and `rv` (randomness) values
   - Use TraceID randomness when explicit `rv` is not provided
   - Update TraceState with new threshold values when sampling decisions change effective probability
   - Propagate TraceState to all outgoing spans

2. **Consistent Probability Sampling**:
   - Replace hash-based probabilistic sampling with OTEP 235 threshold comparison
   - Ensure sampling decisions are consistent: if a trace is sampled with probability P1, it must be sampled with any probability P2 >= P1
   - Use `pkg/sampling` utilities for all threshold calculations and sampling decisions

3. **Policy Coordination**:
   - Coordinate multiple policies to maintain overall consistency
   - Adjust thresholds appropriately when multiple policies apply
   - Ensure final threshold reflects the most restrictive sampling applied

4. **Backward Compatibility**:
   - Maintain existing configuration API and behavior for users not using TraceState
   - Support gradual migration through feature detection
   - Preserve all existing policy types with enhanced consistency

### Rate limiting traces: high-level approach

The tail sampling processor with its existing logic accumulates
batches of unfinished traces, then evaluates them and flushes them
after an elapsed time. We can implement a rate limit at the moment of
flushing by adjusting thresholds.

There is a complication that we will ignore in order to make progress,
which is that incoming spans can be sampled with independent
thresholds, which will cause the simple algorithm described here to
sometimes break traces without further sophistication.

Nevertheless, a simple algorithm can be implemented:

1. Sort traces ascending by randomness value (TraceID or `rv`)
2. Repeat the following steps until the batch meets the rate limit
3. Find the minimum randomness value MR.
4. Remove this trace from the batch; adjust threshold for all spans
   in batch to `max(existing, MR+1)` considering the existing threshold.

### Implementation Considerations

#### Performance Requirements
- TraceState parsing should have minimal impact on trace processing throughput
- Threshold calculations should be cached where possible
- Memory overhead for threshold tracking should be bounded

#### Migration Strategy
- **Feature Detection**: Enable OTEP 235 behavior when TraceState with `th` values is present
- **Configuration Flags**: Add optional configuration to explicitly enable new behavior
- **Dual Mode Operation**: Support both legacy and OTEP 235 modes simultaneously during transition
- **Documentation**: Provide clear migration guidance and examples

#### Error Handling
- Handle malformed or missing TraceState gracefully
- Provide clear error messages for configuration issues
- Maintain processor stability when sampling decisions fail

#### Testing Requirements
- Unit tests for all new threshold-based logic
- Integration tests with various TraceState scenarios
- Performance benchmarks comparing legacy and OTEP 235 modes
- Compatibility tests ensuring existing configurations continue working

## Project phases

### Phase 1 ✅ COMPLETE

Learn the existing code base. Write a document explaining the existing
conditions and structure of the code.

**Deliverables:**
- [x] Memory Bank documentation structure established
- [x] Comprehensive codebase analysis in `memory-bank/codebase-analysis.md`
- [x] Gap analysis identifying specific limitations vs OTEP 235 requirements
- [x] Architecture documentation of existing processor and policy framework
- [x] Integration points identified with pkg/sampling utilities

**Key Findings:**
- Current probabilistic sampler uses incompatible hash-based approach
- Rate limiting lacks threshold adjustment and consistency guarantees
- No systematic TraceState handling infrastructure
- Policy framework suitable for extension but requires coordination mechanism
- pkg/sampling provides all necessary OTEP 235 utilities

### Phase 2

Write a plan with a series of steps which can be executed to update the
code base with new requirements. Define the remaining phases of the project.

**Deliverables:**
- [ ] Detailed implementation plan with specific code changes
- [ ] Policy interface evolution strategy maintaining backward compatibility
- [ ] TraceState integration architecture design
- [ ] Testing strategy for both legacy and OTEP 235 modes
- [ ] Performance benchmarking plan and acceptance criteria
- [ ] Configuration migration guide and examples
- [ ] Definition of Phase 3+ implementation steps

### Phase 3+

To be determined by the agent based on Phase 2 planning.

**Anticipated Phases:**
- **Phase 3**: Core infrastructure (TraceState handling, threshold management)
- **Phase 4**: Policy updates (probabilistic, rate limiting)
- **Phase 5**: Integration and coordination layer
- **Phase 6**: Testing and performance validation
- **Phase 7**: Documentation and examples

## Success Criteria

### Functional Requirements
- [ ] All traces with TraceState `th` values are processed using OTEP 235 algorithms
- [ ] Probabilistic sampling produces consistent decisions across sampling stages
- [ ] Rate limiting maintains sampling consistency through threshold adjustment
- [ ] All existing policy types continue to work with enhanced consistency
- [ ] TraceState values are properly propagated to downstream processors

### Non-Functional Requirements
- [ ] Performance degradation < 5% for existing workloads
- [ ] Memory overhead for threshold tracking < 10% of current usage
- [ ] 100% backward compatibility for existing configurations
- [ ] Comprehensive test coverage for both legacy and OTEP 235 modes

### Documentation Requirements
- [ ] Updated processor documentation with OTEP 235 features
- [ ] Migration guide for existing users
- [ ] Configuration examples for common scenarios
- [ ] Performance tuning recommendations
