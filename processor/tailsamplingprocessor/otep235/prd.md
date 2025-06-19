# Updated tail sampling processor - Complete Project Summary

This document provides a comprehensive summary of the project to update the tail sampling processor in this
repository to use new conventions, based on the OpenTelemetry
consistent probability sampling specifications (OTEP 235).

## Project Overview

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

# Machine Content

The section above was written to kick off the project.  This section
has been edited by the agent as it works through the project.

## Project Status

**Current Phase**: Phase 4 (Policy Algorithm Updates) - Ready to Begin  
**Last Completed**: Phase 3 (Core Infrastructure) - Git Commit: 37a8c14552  
**Overall Progress**: 3 of 7 phases complete (43%)

### Key Achievements

- ✅ **Phase 1**: Comprehensive codebase analysis and requirements gathering
- ✅ **Phase 2**: Detailed implementation plan with 7-phase roadmap  
- ✅ **Phase 3**: Core OTEP 235 infrastructure implemented and validated
- 📋 **Phase 4-7**: Policy updates, integration, testing, and documentation (planned)

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

### Phase 1 ✅ COMPLETE (Initial Analysis)

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

### Phase 2 ✅ COMPLETE (Implementation Planning)

Write a plan with a series of steps which can be executed to update the
code base with new requirements. Define the remaining phases of the project.

**Deliverables:**
- [x] Detailed implementation plan with specific code changes in `memory-bank/phase2-implementation-plan.md`
- [x] TraceState integration architecture design
- [x] Performance benchmarking plan and acceptance criteria
- [x] Configuration migration guide and examples
- [x] Definition of Phase 3-7 implementation steps
- [x] Simplified algorithm replacement strategy (no dual-mode complexity)
- [x] Testing infrastructure requirements defined

**Key Decisions:**
- Algorithm replacement approach (no cross-restart sampling consistency required)
- Single implementation path using OTEP 235 throughout
- Backward compatibility through functional equivalence
- Direct pkg/sampling API usage for all threshold operations

### Phase 3 ✅ COMPLETE (Core Infrastructure) - Git Commit: 37a8c14552

Implement core OTEP 235 infrastructure including TraceState handling and threshold management.

**Deliverables:**
- [x] TraceData structure enhancement with OTEP 235 fields (RandomnessValue, FinalThreshold, TraceStatePresent)
- [x] TraceStateManager implementation for parsing and threshold extraction
- [x] Processor integration for OTEP 235 field initialization
- [x] pkg/sampling API integration (OpenTelemetryTraceState, RandomnessFromTraceID, etc.)
- [x] Comprehensive integration tests for OTEP 235 functionality
- [x] Backward compatibility validation (all existing tests updated and passing)
- [x] Production-ready error handling and edge case management

**Key Implementation:**
```go
// Enhanced TraceData structure
type TraceData struct {
    // ...existing fields...
    RandomnessValue   sampling.Randomness    // TraceID randomness or explicit rv
    FinalThreshold    *sampling.Threshold    // Final threshold after all policies
    TraceStatePresent bool                   // Track if any spans have TraceState
}

// TraceStateManager for OTEP 235 operations
type TraceStateManager struct{}
// - ParseTraceState(trace *TraceData)
// - InitializeTraceData(ctx, traceID, trace)
// - ExtractThreshold/RandomnessValue methods
```

**Phase 3 Impact:**
- Foundation established for all policy algorithm updates
- TraceState parsing working correctly with fallback to TraceID randomness
- Processor seamlessly initializes OTEP 235 fields for each trace
- All existing functionality preserved while adding OTEP 235 support

### Phase 4 📋 NEXT (Policy Algorithm Updates)

Update individual policies to use OTEP 235 threshold-based algorithms instead of legacy hash-based approaches.

**Planned Deliverables:**
- [ ] Replace probabilistic policy hash-based algorithm with OTEP 235 threshold comparison
- [ ] Implement threshold-based rate limiting algorithm from PRD specification
- [ ] Update numeric tag policy to maintain OTEP 235 consistency
- [ ] Update other policies (latency, error, composite) to coordinate with threshold system
- [ ] Comprehensive policy-level testing with OTEP 235 scenarios
- [ ] Performance validation for each updated policy

**Implementation Focus:**
- Use TraceData.RandomnessValue and pkg/sampling.ShouldSample() for all sampling decisions
- Implement PRD rate limiting algorithm with threshold adjustment
- Ensure all policies coordinate to maintain sampling consistency
- Preserve existing configuration APIs and expected behaviors

### Phase 5 📋 PLANNED (Integration and Coordination)

Implement policy coordination and TraceState propagation throughout the processor.

**Planned Deliverables:**
- [ ] Policy coordination mechanism for threshold management
- [ ] TraceState propagation to outgoing spans with updated thresholds
- [ ] Cross-policy consistency validation
- [ ] Integration testing with realistic multi-policy scenarios
- [ ] Performance optimization for TraceState operations

### Phase 6 📋 PLANNED (Testing and Performance Validation)

Comprehensive testing and performance validation of the complete OTEP 235 implementation.

**Planned Deliverables:**
- [ ] Complete test suite with >95% coverage for OTEP 235 paths
- [ ] Performance benchmarks comparing legacy vs OTEP 235 implementation
- [ ] Load testing with various TraceState scenarios
- [ ] Backward compatibility verification across all policy combinations
- [ ] Memory and CPU impact analysis
- [ ] Performance regression tests

### Phase 7 📋 PLANNED (Documentation and Examples)

Final documentation, examples, and migration guidance for users.

**Planned Deliverables:**
- [ ] Updated processor documentation with OTEP 235 features
- [ ] Migration guide explaining algorithm changes and expected impacts
- [ ] Configuration examples for common OTEP 235 scenarios
- [ ] Performance tuning recommendations
- [ ] Troubleshooting guide for TraceState-related issues
- [ ] Integration examples with other OTEP 235-compliant components

## Success Criteria

### Phase 3 Completed ✅

- [x] TraceData enhanced with OTEP 235 fields (RandomnessValue, FinalThreshold, TraceStatePresent)
- [x] TraceStateManager implemented with pkg/sampling integration
- [x] Core infrastructure integrated into processor pipeline
- [x] Integration tests validate OTEP 235 field initialization and TraceState parsing
- [x] All existing tests updated and passing (backward compatibility maintained)
- [x] Production-ready error handling and edge case management

### Functional Requirements (Phase 4-7)

- [ ] All traces with TraceState `th` values are processed using OTEP 235 algorithms
- [ ] Probabilistic sampling produces consistent decisions across sampling stages
- [ ] Rate limiting maintains sampling consistency through threshold adjustment
- [ ] All existing policy types continue to work with enhanced consistency
- [ ] TraceState values are properly propagated to downstream processors

### Non-Functional Requirements

- [ ] Performance degradation < 5% for existing workloads
- [ ] Memory overhead for threshold tracking < 10% of current usage
- [x] 100% backward compatibility for existing configurations (Phase 3 validated)
- [ ] Comprehensive test coverage for both legacy and OTEP 235 modes

### Documentation Requirements

- [ ] Updated processor documentation with OTEP 235 features
- [ ] Migration guide for existing users
- [ ] Configuration examples for common scenarios
- [ ] Performance tuning recommendations

## Implementation Decision: Simplified Algorithm Replacement

**Date**: June 19, 2025

**Decision**: Replace the existing hash-based probabilistic sampling algorithm with OTEP 235 threshold-based sampling across all configurations. We are not preserving sampling consistency across processor restarts, which allows us to change the selection algorithm arbitrarily while maintaining functional compatibility.

**Rationale**:

- Eliminates complexity of dual-mode operation and feature detection
- Maintains all existing functionality and configuration options
- Provides immediate OTEP 235 compliance benefits for all users
- Preserves same effective sampling rates with improved consistency
- processor/probabilisticsamplerprocessor has demonstrated use of
  OTEP 235 explicit randomness value to preserve backwards compatibility
  with legacy hash function
- We can address this later

**Impact**:

- Same trace may receive different sampling decision after processor restart (acceptable)
- All configurations continue to work with same expected behavior
- TraceState handling becomes automatic and transparent
- Single implementation path reduces maintenance burden

## Current Project State and Next Steps

### Infrastructure Ready ✅

Phase 3 has successfully established the foundation for OTEP 235 compliance:

- **TraceStateManager**: Fully operational with pkg/sampling integration
- **Enhanced TraceData**: Includes RandomnessValue, FinalThreshold, and TraceStatePresent fields
- **Processor Integration**: OTEP 235 fields automatically initialized for each trace
- **Test Infrastructure**: Comprehensive integration tests validate OTEP 235 functionality
- **Backward Compatibility**: All existing tests pass with OTEP 235 infrastructure in place

### Ready for Phase 4: Policy Updates

The next phase focuses on updating individual policies to use OTEP 235 algorithms:

1. **Probabilistic Policy**: Replace hash-based sampling with threshold comparison using `pkg/sampling.ShouldSample()`
2. **Rate Limiting Policy**: Implement PRD algorithm for threshold-based rate limiting with trace sorting
3. **Policy Coordination**: Ensure multiple policies work together to maintain sampling consistency
4. **Testing**: Validate each policy update with comprehensive test scenarios

### Long-term Roadmap

- **Phase 5-6**: Integration, testing, and performance validation
- **Phase 7**: Documentation and examples for users
- **Expected Completion**: Full OTEP 235 compliance with maintained backward compatibility

The project is well-positioned for successful completion with a solid foundation already implemented and tested.
