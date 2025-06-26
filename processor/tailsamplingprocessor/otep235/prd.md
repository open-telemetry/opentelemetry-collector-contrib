# Updated tail sampling processor

This document provides a comprehensive summary of the project to
update the tail sampling processor in this repository to use new
conventions, based on the OpenTelemetry consistent probability
sampling specifications (OTEP 235) and composable sampling design
(OTEP 250).

## Project Overview

In the processor/tailsamplingprocessor/otep235-context directory, find
several relevant documents and artifacts:

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

Note that there is a helper library for OTEP 235 in pkg/sampling which
has been incorporated into the intermediate span sampler processor in
processor/probabilisticsamplerprocessor, which is a span-oriented
sampler, not a trace-level sampler.

## Definition of done

After this project is complete, the tailsampling processor will be
fully compatible with OTEP 235, meaning that changes of effective
probability for a trace correctly changes the sampling threshold in
individual span tracestate values.

## New tailsamplingprocessor requirements

The OpenTelemetry tracestate threshold value known as T (encoded `th`)
directly indicates effective sampling probability. Changes in
effective probability must be achieved in a way consistent with this
definition. T is a rejection threshold 0 equals 100% sampling (none
rejected). The specification (and probability laws) dictate that
threshold can never decrease; however it can rise to record the
effects of additional sampling.

OTEP 250 demonstrates how to implement a rule-based head sampler which
maintains consistent probability sampling, considerate of upstream
sampling decisions. Many of the aspects of the updated processor will
be similar to the SDK samplers, however establishing rate limits is
different for tail sampling compared with head sampling.

### Rate limiting traces: high-level approach

The tail sampling processor with its existing logic accumulates
batches of unfinished traces, then evaluates them and flushes them
after an elapsed time. We can implement a rate limit at the moment of
flushing by adjusting thresholds.

Nevertheless, a simple algorithm can be implemented:

1. Sort traces ascending by randomness value (56 bits of TraceID or `rv`)
2. Repeat the following steps until the batch meets the rate limit
3. Find the minimum randomness value MR.
4. Remove this trace from the batch; adjust threshold for all spans
   according to Bottom-K sampling estimator which uses. See
   [bottom-k.pdf](./bottom-k.pdf) or [ACM link](https://dl.acm.org/doi/10.1145/1269899.1254926).

Whenever we need to store or output at most N items, whether for space
reasons or rate limiter configuration, we will sort the set by
randomness value and maintain the N+1 largest values (e.g., by
selection sort or by maintaining a sorted set). Choosing the largest
values maximimizes trace completeness under OTEP 235. Note that the
bottom-k algorithm is based on smallest hash-values; we will invert
the logic substituting (max_adjusted_count - randomess_value) as the
equivalent min-hash value in the bottom-k adjusted-count formula to
calculate updated thresholds.

#### Reservoir implementation

The existing component is structured with a ticker interval and a
maximum number of resident traces awaiting decisions. Occasionally
there is a need to reduce the number of pending traces. We can divide
the `decision_wait` parameter (e.g., 30s) into a fixed number buckets
(e.g., 10x 3s buckets) aligned to an arbitrary start time. All traces
in a bucket are decided at bucket end-time.

To keep it simple, we will assign each bucket a fixed size limit
(e.g., 1/10 of the total capacity for 10 buckets); for a bucket with
size K we will keep K+1 traces per bucket to maintain the bottom-k
estimate.

At the end-time of a bucket, the entire batch of traces will be
evaluated.  Rate-limiter samplers will return AlwaysSample with an
attribute indicating their policy name during evaluation. After the
batch has been processed, it will be reservoir-processed in a second
pass.  Considering the rate limit (determined from the policy name), a
number of traces will drop and the remaining ones (matching the
policy) will have their thresholds raised accordingly.

#### Late-arriving spans

The component provides an additional feature making it possible to
remember a sampling decision after the decision_wait time to correctly
sample late-arriving spans. This feature is orthogonal and may need to
be re-evaluated.  It's not clear why the current decision cache needs
both a positive and negative form; instead it should probably have
per-time bucketing allowing whole groups of decided trace-IDs to be
dropped rather than maintaining a fine-grain data structure.

