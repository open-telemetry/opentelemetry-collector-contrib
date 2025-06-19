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

## Definition of done

After this project is complete, the tailsampling processor will be
fully compatible with OTEP 235, meaning that changes of effective
probability for a trace correctly changes the sampling threshold in
individual span tracestate values.

## New tailsampingprocessor requirements

The OpenTelemetry tracestate thresohld value known as T (encoded `th`)
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
batches of unfinished traces, then evalutes them and flushes them
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

## Project phases

### Phase 1

Learn the existing code base. Write a document explaining the existing
conditions and structure of the code.

### Phase 2

Write a plan with a series of steps which can be executed to update the
code base with new requirements. Define the remaining phases of the project

### Phase 3+

To be determined by the agent.
