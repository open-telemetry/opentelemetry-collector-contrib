<!--- Hugo front matter used to generate the website version of this page:
linkTitle: Probability Sampling
--->

# TraceState: Probability Sampling

**Status**: [Development](../document-status.md)

<details>
<summary>Table of Contents</summary>

<!-- toc -->

- [Overview](#overview)
- [Definitions](#definitions)
  * [Sampling Probability](#sampling-probability)
  * [Consistent Sampling Decision](#consistent-sampling-decision)
  * [Rejection Threshold (`T`)](#rejection-threshold-t)
  * [Randomness Value (`R`)](#randomness-value-r)
- [Approach and Terminology](#approach-and-terminology)
  * [Decision algorithm](#decision-algorithm)
  * [Sampling stages](#sampling-stages)
  * [Sampling base cases](#sampling-base-cases)
  * [Sampling related terms](#sampling-related-terms)
- [Tracestate handling requirements](#tracestate-handling-requirements)
  * [General requirements](#general-requirements)
  * [Parent/Child threshold](#parentchild-threshold)
    + [Independent parent/child threshold](#independent-parentchild-threshold)
    + [Parent-based threshold](#parent-based-threshold)
  * [Downstream threshold](#downstream-threshold)
    + [Equalizing downstream sampler](#equalizing-downstream-sampler)
    + [Proportional downstream sampler](#proportional-downstream-sampler)
- [Migration to consistent probability samplers](#migration-to-consistent-probability-samplers)
- [Algorithms](#algorithms)
  * [Converting floating-point probability to threshold value](#converting-floating-point-probability-to-threshold-value)
  * [Converting integer threshold to a `T`-value](#converting-integer-threshold-to-a-t-value)
  * [Testing randomness vs threshold](#testing-randomness-vs-threshold)
  * [Converting threshold to a sampling probability](#converting-threshold-to-a-sampling-probability)
  * [Converting threshold to an adjusted count (sampling rate)](#converting-threshold-to-an-adjusted-count-sampling-rate)

<!-- tocstop -->

</details>

## Overview

Sampling is an important lever to reduce the costs associated with collecting and processing telemetry data.
It enables you to choose a representative set of items from an overall population.

There are two key aspects for sampling of tracing data.
The first is that sampling decisions can be made independently for *each* span in a trace.
The second is that sampling decisions can be made at multiple points in the telemetry pipeline.
For example, the sampling decision for a span at span creation time could have been to **keep** that span, while the downstream sampling decision for the *same* span at a later stage (say in an external process in the data collection pipeline) could be to **drop** it.

For each of the above aspects, if we don't make **consistent** sampling decisions, we will end up with traces that are unusable and do not contain a coherent set of spans, because of the independent sampling decisions.
Instead, we want sampling decisions to be made in a **consistent** manner so that we can effectively reason about a trace.

This specification describes how to achieve consistent sampling decisions using a mechanism called **Consistent Probability Sampling**.
To achieve this, it uses two key building blocks.
The first is a common source of randomness (`R`) that is available to all participants, which includes a set of tracers and collectors.
This can be either an explicit randomness value expressed in the OpenTelemetry TraceState `rv` sub-key or taken from the trailing 7 bytes of the TraceID.
The second is a concept of a rejection threshold (`T`). This is derived directly from a participant's sampling rate and is expressed in the OpenTelemetry TraceState `th` sub-key for sampled spans.
This proposal describes how these two values should be propagated and how participants should use them to make sampling decisions.

For more details about this specification, see [OTEP
235](../../oteps/trace/0235-sampling-threshold-in-trace-state.md) and
[OTEP 235](../../oteps/trace/oteps/trace/0250-Composite_Samplers.md).

For details about encoding probability sampling information in
OpenTelemetry tracing data, in particular the OpenTelemetry TraceState
`rv` and `th` values, see [Tracestate
handling](./tracestate-handling.md). Hereafter, we use the variable
`R` and `T` to represent these quantities in making sampling
decisions.

## Definitions

### Sampling Probability

Sampling probability is the likelihood that a span will be *kept*. Each participant can choose a different sampling probability for each span.
For example, if the sampling probability is 0.25, around 25% of the spans will be kept.

In OpenTelemetry, sampling probability is valid in the range 2^-56 through 1.
The value 56 appearing in this expression corresponds with 7 bytes of randomness (i.e., 56 bits) which are specified for W3C Trace Context Level 2 TraceIDs.
Note that the zero value is not defined and that "never" sampling is not a form of probability sampling.

### Consistent Sampling Decision

A consistent sampling decision means that a positive sampling decision made for a particular span with probability p1 necessarily implies a positive sampling decision for any span belonging to the same trace if it is made with probability p2 >= p1.

### Rejection Threshold (`T`)

This is a 56-bit value directly derived from the sampling probability.
One way to think about this is that this is the number of spans that would be *dropped* out of 2^56 considered spans.

You can derive the rejection threshold from the sampling probability as follows:

```
Rejection_Threshold = (1 - Sampling_Probability) * 2^56.
```

For example, if the sampling probability is 100% (keep all spans), the rejection threshold is 0.

Similarly, if the sampling probability is 1% (drop 99% of spans), the rejection threshold with 5 digits of precision would be (1-0.01) * 2^56 ≈ 71337018784743424 = 0xfd70a400000000.

We refer to this rejection threshold as `T`.
When a Span or Context is sampled, the sampler's effective `T` is encoded in the OpenTelemetry TraceState `th` sub-key to indicate its sampling probability.
A span sampled at 1% in this case will have an OpenTelemetry TraceState value `ot=th:fd70a4`, because trailing zeros are removed from the encoding.

See [tracestate handling](./tracestate-handling.md#sampling-threshold-value-th) for more examples.

### Randomness Value (`R`)

A common random value (that is known or propagated to all participants) is the main ingredient that enables consistent probability sampling.
Each participant can compare this value (`R`) with their rejection threshold (`T`) to make a consistent sampling decision across an entire trace (or even across a group of traces).

This proposal supports two sources of randomness:

- **An explicit source of randomness**: OpenTelemetry supports a *random* (or pseudo-random) 56-bit value known as explicit trace randomness. This can be propagated through the OpenTelemetry TraceState `rv` sub-key and is stored in the Span's TraceState field.
- **Using TraceID as a source of randomness**: OpenTelemetry supports using the [least-significant 56 bits of the TraceID as the source of randomness, as specified in W3C Trace Context Level 2][W3CCONTEXTTRACEID]. This can be done if the root Span's Trace SDK knows that the TraceID has been generated in a random or pseudo-random manner.

[W3CCONTEXTTRACEID]: https://www.w3.org/TR/trace-context-2/#randomness-of-trace-id

See [tracestate handling](./tracestate-handling.md#explicit-randomness-value-rv) for details about encoding randomness values.

## Approach and Terminology

### Decision algorithm

Given the above building blocks, let's look at how a participant can make consistent sampling decisions.
For this, two values MUST be present:

1. In the `SpanContext`, the common source of randomness: the 56-bit randomness value (`R`).
2. From sampler configuration, the rejection threshold: the 56-bit threshold value (`T`).

If `R` >= `T`, *keep* the span, else *drop* the span.

### Sampling stages

The two sampling aspects mentioned in the overview are referred to as
stages of a sampling strategy:

1. Parent/Child sampling. These decisions are made for spans inside an SDK, synchronously, during the life of a trace. These decisions are based on live Contexts.
2. Downstream sampling. These sampling decisions happen for spans on the collection path, after they finish, and may happen at multiple locations in a collection pipeline. These decisions are based on historical Contexts.

We recognize these stages as two dimensions in which Sampling
decisions are made.  In Parent/Child sampling, there is a progression
in the direction of causality, from parent to child forward in time.
In Downstream sampling, there is a progression from collector to
collector forward in time.  Considering a finished span at a moment in
time, it will have been sampled once by a Parent/Child sampler and
zero or more times by a Downstream sampler.

In both cases, the "previous" sampler refers to the sampling stage
that precedes it in time (i.e., the parent or upstream sampler).

```mermaid
graph TD
    subgraph "parent/child samplers"
        direction LR
        Root["root span"] --> Child1["child span"]
        Child1 --> Child2["child span"]
    end

    subgraph "downstream samplers"
        Root --> LC1["frontend agent"]
        LC1 --> LC2["frontend gateway"]

        Child1 --> RC1["backend agent"]
        Child2 --> RC1
        RC1 --> RC2["backend gateway"]
    end

    LC2 --> FC["destination service"]
    RC2 --> FC

    classDef span fill:#a7a,stroke:#333,stroke-width:2px;
    classDef collector fill:#77a,stroke:#33c,stroke-width:1px;

    class Root,Child1,Child2 span;
    class LC1,LC2,RC1,RC2,FC collector;
```

### Sampling base cases

The CompositeSampler is meant to support combining multiple sampling
rules into one using built-in ComposableSamplers as the base cases.

Within the category of Parent/Child sampling, these are the primary
base cases:

- Root: The Root sampling decision is the first decision in both
  sampling dimensions, made with no prior threshold. The Root sampling
  decision is the only case where it is permitted to modify the
  explicit trace randomness value for a Context.
- Parent-based: When using parent-based sampling at a child, we expect
  the child's decision to match the parent's decision.  The parent and
  child will have matching threshold values.
- Consistent probability: A probability sampler (e.g., TraceIDRatio)
  makes an independent sampling decision.  A consistent probability
  sampling decision ignores the parent's sampling threshold (if any).
  This case covers always-on and always-off sampling behaviors.

### Sampling related terms

Some terms are not about one specific sampler, but about the approach:

- Head: **Caution: This term has multiple uses.** This may refer to
  the sampling decision an SDK makes when starting a new span, for
  example "head sampling decisions can only consider information
  present during span creation" (from the [API specification for Span
  Link](./api.md#link).  This can also refer to a distributed tracing
  setup, in which all child samplers use a parent-based sampler, for
  example "most distributed tracing configurations use head sampling".
- Tail: This mode of sampling implies that Downstream samplers are
  involved in the decision. Sometimes this refers to sampling of
  individual spans on the collection path (known as "intermediate"
  sampling), but usually it refers to whole-trace sampling decisions
  made downstream after assembling multiple spans into a single data
  set.  Tail sampling may be combined with Head sampling, of course,
  and may or may not contribute unequal probabilities across spans
  within a trace.

The term **adjusted count** refers to the mathematical inverse
(reciprocal) of the sampling probability, the expected number of times
an item should be counted, for it to be representative of the
population. Adjusted counts are useful as an estimate of the number
of spans there would be without sampling.

The term **span-to-metrics** refers to the use of adjusted count in
a telemetry system to derive metrics from tracing spans using adjusted
count information. The specifications for handling tracestate and
managing threshold information in samplers ensure that adjusted counts
are reliable primarily for this purpose.

## Tracestate handling requirements

This is a supplemental guideline. See [the SDK
specification](./sdk.md) for the specific requirements of each
built-in sampler.

The rejection threshold represents the maximum threshold that was
applied in all previous consistent sampling stages, including the
parent/child and downstream samplers.

See [SDK requirements for trace
randomness](./sdk.md#sampling-requirements), which covers potentially
inserting explicit trace randomness in the Context.

Refer to [TraceState handling](./tracestate-handling.md) for examples
showing how `th` and `rv` are encoded.

### General requirements

All sampling stages that modify the effective sampling threshold must
update the sampling threshold by re-encoding the OpenTelemetry
TraceState, to maintain the correct statistical interpretation.

For a conditional sampling stages (e.g. rule-based samplers) to remain
consistent, they must not condition the provided threshold on the
randomness value or a dependent source of randomness (it can use
*independent* randomness).

Sampling stages that yield spans with unknown sampling probability,
including parent-based samplers when they encounter a Context with
no parent thresohld, must erase the OpenTelemetry threshold
value in their output.

Sampling stages should check for consistency when it is a simple test,
and they should erase the threshold when it is apparently
inconsistent.  For example, a parent-based sampler that receives and
propagates a sampled context with threshold should check that the
sampled flag matches the expression `rv >= th`, otherwise it should
erase the threshold.

If randomness was derived from an explicit randomness value, the
identical `rv` value must be set in the outgoing OpenTelemetry
TraceState.

### Parent/Child threshold

The Parent/Child sampler initializes the consistent probability
threshold. This can be achieved by using one of the built-in samplers,
either as a base case or using the logic of a ComposableSampler.

#### Independent parent/child threshold

Because there is no parent threshold, a root sampler always makes an
independent decision.  Child samplers may be configured with
independent samplers, although it can lead to incomplete traces.

The independent Parent/Child sampler base cases (i.e., AlwaysOn,
TraceIDRatio) cases encode a fixed threshold based on their
sampling probability.

#### Parent-based threshold

Parent-based samplers propagate the incoming threshold as the outgoing
threshold, subject to the general requirements:

- If a sampled incoming threshold is apparently inconsistent, erase it
- If a sampled incoming threshold is absent, ensure there is no
  outgoing threshold.

### Downstream threshold

Downstream samplers are able to raise the applied threshold, but they
cannot lower.  For a Downstream sampler to lower a threshold equates
with retroactively raising an earlier stage's sampling probability.
Stated another way, downstream samplers are permitted to decrease
sampling probability, but to raise sampling probability would
introduce a statistical error.

Two downstream probability sampling have standard terminology,
discussed next.

#### Equalizing downstream sampler

An equalizing downstream sampler aims to make all spans have an equal
threshold after passing the stage.  This stage is more selective when
earlier stages have been less selective, resulting in spans with equal
probability.

When an equalizing downstream sampler configured with threshold `T_d`
considers a span with threshold `T_s` for randomness value `R`:

- If `T_s > T_d`, the outbound threshold is unchanged, equal to
  `T_s`. An equalizing probability sampler cannot lower the threshold,
  therefore it cannot equalize probability in this case.
- If `R >= T_d` the span is selected using outbound threshold `T_d`.
- Otherwise, `R < T_d` indicates that the span is not sampled.

#### Proportional downstream sampler

A proportional downstream sampler aims to multiply the incoming
probability with a configured multiplier. By raising the threshold
uniformly, this stage of sampler promises to reduce traffic by a fixed
amount, despite their arriving thresholds.

When a proportional downstream sampler configured with probability `p`
considers a span with threshold `T_s` for randomness value `R`, first
it computes the product threshold `T_o`:

```
T_o = ProbabilityToThreshold(p * ThresholdToProbability(T_s))
```

- If `T_o` is smaller than the minimum probability threshold, the span
  is not sampled.
- If `R >= T_o` the span is selected using outbound threshold `T_o`.
- Otherwise, `R < T_o` indicates that the span is not sampled.

## Migration to consistent probability samplers

The OpenTelemetry specification for TraceIdRatioBased samplers was not completed until after the SDK specification was declared stable, and the exact behavior of that sampler was left unspecified.
The current specification addresses this behavior.

As the OpenTelemetry TraceIdRatioBased sampler changes definition, users must consider how to avoid incomplete traces due to inconsistent sampling during the transition between old and new logic.

The original TraceIdRatioBased sampler specification gave a workaround for the underspecified behavior, that it was safe to use for root spans: "It is recommended to use this sampler algorithm only for root spans (in combination with [`ParentBased`](./sdk.md#parentbased)) because different language SDKs or even different versions of the same language SDKs may produce inconsistent results for the same input."

To avoid inconsistency during this transition, users SHOULD follow this guidance until all Trace SDKs in a system have been upgraded to modern Trace randomness requirements based on W3C Trace Context Level 2.
Users can verify that all Trace SDKs have been upgraded when all Spans in their system have the Trace random flag set (in Span flags).
To assist with this migration, the TraceIdRatioBased Sampler issues a warning statement the first time it presumes TraceID randomness for a Context where the Trace random flag is not set.

## Algorithms

The `T` and `R` values may be represented and manipulated in a variety of forms depending on the capabilities of the processor and needs of the implementation.
As 56-bit values, they are compatible with byte arrays and 64-bit integers, and can also be manipulated with 64-bit floating point with a truly negligible loss of precision.

The following examples are in Python3.
They are intended as examples only for clarity, and not as a suggested implementation.

### Converting floating-point probability to threshold value

Threshold values are encoded with trailing zeros removed, which allows for variable precision.
This can be accomplished by rounding, and there are several practical ways to do this with built-in string formatting libraries.

With up to 56 bits of precision available, implementations that use built-in floating point number support will be limited by the precision of the underlying number support.
One way to encode thresholds uses the IEEE 754-2008-standard hexadecimal floating point representation as a simple solution.

```python
import math

# ProbabilityToThresholdWithPrecision assumes the probability value is in the range
# [2^-56, 1] and precision is in the range [1, 13], which is the maximum for a
# IEEE-754 double-width float value.
def probability_to_threshold_with_precision(probability, precision):
    if probability == 1:
        # Special case
        return "0"

    # Raise precision by the number of leading 'f' digits.
    _, exp = math.frexp(probability)
    # Precision is limited to 12 so that there is at least one digit of precision
    # in the final [:precision] statement below.
    precision = max(1, min(12, precision + exp // -4))

    # Change the probability to 1 + rejection probability = 1 + 1 - probability,
    # i.e., modify the range (0, 1] into the range [1, 2).
    rejection_prob = 2 - probability

    # To ensure rounding correctly below, add an offset equal to half of the
    # final digit of precision in the corresponding representation.
    rejection_prob += math.ldexp(0.5, -4 * precision)

    # The expression above technically can't produce a number >= 2.0 because
    # of the compensation for leading Fs. This gives additional safety for
    # the hex_str[4:][:-3] expression below which blindly drops the exponent.
    if rejection_prob >= 2.0:
        digits = "fffffffffffff"
    else:
        # Use float.hex() to get hexadecimal representation
        hex_str = rejection_prob.hex()

        # The hex representation for values between 1 and 2 looks like '0x1.xxxxxxxp+0'
        # Extract the part after '0x1.' (4 bytes) and before 'p' (3 bytes)
        digits = hex_str[4:][:-3]

    assert len(digits) == 13
    # Remove trailing zeros
    return digits[:precision].rstrip('0')
```

Note the use of `math.frexp(probability)` used to adjust precision using the base-2 exponent of the probability argument.
This makes the configured precision apply to the significant digits of the threshold for probabilities near zero.
Note that there is not a symmetrical adjustment made for values near unit probability, as we do not believe there is a practical use for sampling very precisely near 100%.

To translate directly from floating point probability into a 56-bit unsigned integer representation using `math.Round()` and shift operations, see the [OpenTelemetry Collector-Contrib `pkg/sampling` package][PKGSAMPLING] package.
This package demonstrates how to directly calculate integer thresholds from probabilities.

[PKGSAMPLING]: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/sampling/README.md

OpenTelemetry SDKs are recommended to use 4 digits of precision by default.
The following table shows values computed by the method above for 1-in-N probability sampling, with precision 3, 4, and 5.

<!--- Program at https://go.dev/play/p/7eLM6FkuoA5 (includes function above) generates the table below --->
| 1-in-N  | Input probability  | Threshold (precision 3, 4, 5)      | Actual probability (precision 3, 4, 5)                                       | Exact Adjusted Count (precision 3, 4, 5)                              |
|---------|--------------------|------------------------------------|------------------------------------------------------------------------------|-----------------------------------------------------------------------|
| 1       | 1                  | 0<br/>0<br/>0                      | 1<br/>1<br/>1                                                                | 1<br/>1<br/>1                                                         |
| 2       | 0.5                | 8<br/>8<br/>8                      | 0.5<br/>0.5<br/>0.5                                                          | 2<br/>2<br/>2                                                         |
| 3       | 0.3333333333333333 | aab<br/>aaab<br/>aaaab             | 0.333251953125<br/>0.3333282470703125<br/>0.33333301544189453                | 3.0007326007326007<br/>3.00004577706569<br/>3.0000028610256777        |
| 4       | 0.25               | c<br/>c<br/>c                      | 0.25<br/>0.25<br/>0.25                                                       | 4<br/>4<br/>4                                                         |
| 5       | 0.2                | ccd<br/>cccd<br/>ccccd             | 0.199951171875<br/>0.1999969482421875<br/>0.19999980926513672                | 5.001221001221001<br/>5.0000762951094835<br/>5.0000047683761295       |
| 8       | 0.125              | e<br/>e<br/>e                      | 0.125<br/>0.125<br/>0.125                                                    | 8<br/>8<br/>8                                                         |
| 10      | 0.1                | e66<br/>e666<br/>e6666             | 0.10009765625<br/>0.100006103515625<br/>0.10000038146972656                  | 9.990243902439024<br/>9.99938968568813<br/>9.999961853172863          |
| 16      | 0.0625             | f<br/>f<br/>f                      | 0.0625<br/>0.0625<br/>0.0625                                                 | 16<br/>16<br/>16                                                      |
| 100     | 0.01               | fd71<br/>fd70a<br/>fd70a4          | 0.0099945068359375<br/>0.010000228881835938<br/>0.009999990463256836         | 100.05496183206107<br/>99.99771123402633<br/>100.00009536752259       |
| 1000    | 0.001              | ffbe7<br/>ffbe77<br/>ffbe76d       | 0.0010004043579101562<br/>0.0009999871253967285<br/>0.000999998301267624     | 999.5958055290753<br/>1000.012874769029<br/>1000.0016987352618        |
| 10000   | 0.0001             | fff972<br/>fff9724<br/>fff97247    | 0.00010001659393310547<br/>0.00010000169277191162<br/>0.00010000006295740604 | 9998.340882002383<br/>9999.830725674266<br/>9999.99370426336          |
| 100000  | 0.00001              | ffff584<br/>ffff583a<br/>ffff583a5 | 9.998679161071777e-06<br/>1.00000761449337e-05<br/>1.0000003385357559e-05    | 100013.21013412817<br/>99999.238556461<br/>99999.96614643588          |
| 1000000 | 0.000001              | ffffef4<br/>ffffef39<br/>ffffef391 | 9.98377799987793e-07<br/>1.00000761449337e-06<br/>9.999930625781417e-07      | 1.0016248358208955e+06<br/>999992.38556461<br/>1.0000069374699865e+06 |

### Converting integer threshold to a `T`-value

To convert a 56-bit integer rejection threshold value to the `T` representation, emit it as a hexadecimal value (without a leading '0x'), optionally with trailing zeros omitted:

```py
if tvalue == 0:
  add_otel_trace_state('th:0')
else:
  h = hex(tvalue).rstrip('0')
  # remove leading 0x
  add_otel_trace_state('th:'+h[2:])
```

### Testing randomness vs threshold

Given randomness and threshold as 64-bit integers, a sample should be taken if randomness is greater than or equal to the threshold.

```
shouldSample = (randomness >= threshold)
```

### Converting threshold to a sampling probability

The sampling probability is a value from 0.0 to 1.0, which can be calculated using floating point by dividing by 2^56:

```py
# embedded _ in numbers for clarity (permitted by Python3)
maxth = 0x100_0000_0000_0000  # 2^56
prob = float(maxth - threshold) / maxth
```

### Converting threshold to an adjusted count (sampling rate)

The adjusted count indicates the approximate quantity of items from the population that this sample represents. It is equal to `1/probability`.

```py
maxth = 0x100_0000_0000_0000  # 2^56
adjCount = maxth / float(maxth - threshold)
```

Adjusted count is not defined for spans that were obtained via non-probabilistic sampling (a sampled span with no `th` value).
