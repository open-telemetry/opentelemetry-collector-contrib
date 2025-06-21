# Composite Samplers Proposal

This proposal addresses head-based sampling as described by the [Open Telemetry SDK](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/sdk.md#sampling).
It introduces additional _composite samplers_.
Composite samplers use other samplers (_delegates_ or _children_) to make sampling decisions.
The composite samplers invoke the delegate samplers, but eventually make the final call.

The new samplers proposed here have been designed to work with Consistent Probability Samplers. For detailed description of this concept see [probability sampling (OTEP 235)](https://github.com/open-telemetry/oteps/blob/main/text/trace/0235-sampling-threshold-in-trace-state.md).
Also see Draft PR 3910 [Probability Samplers based on W3C Trace Context Level 2](https://github.com/open-telemetry/opentelemetry-specification/pull/3910).

**Table of content:**

- [Motivation](#motivation)
- [The Goal](#the-goal)
  - [Example](#example)
- [Proposed Samplers](#proposed-samplers)
  - [New API](#new-api)
    - [GetSamplingIntent](#getsamplingintent)
    - [Required Arguments for GetSamplingIntent](#required-arguments-for-getsamplingintent)
    - [Return Value](#return-value)
    - [Requirements for the basic samplers](#requirements-for-the-basic-samplers)
    - [Constructing SamplingResult](#constructing-samplingresult)
  - [ConsistentRuleBased](#consistentrulebased)
    - [Predicate](#predicate)
      - [SpanMatches](#spanmatches)
      - [Required Arguments for Predicates](#required-arguments-for-predicates)
    - [Required Arguments for ConsistentRuleBased](#required-arguments-for-consistentrulebased)
  - [ConsistentParentBased](#consistentparentbased)
  - [ConsistentAnyOf](#consistentanyof)
  - [ConsistentRateLimiting](#consistentratelimiting)
    - [Required Arguments for ConsistentRateLimiting](#required-arguments-for-consistentratelimiting)
- [Summary](#summary)
  - [Example - sampling configuration](#example---sampling-configuration)
  - [Limitations](#limitations)
  - [Prototyping](#prototyping)
- [Prior Art](#prior-art)

## Motivation

The need for configuring head sampling has been explicitly or implicitly indicated in several discussions, both within the [Sampling SIG](https://docs.google.com/document/d/1gASMhmxNt9qCa8czEMheGlUW2xpORiYoD7dBD7aNtbQ) and in the wider community.
Some of the discussions are going back a number of years, see for example

- issue [173](https://github.com/open-telemetry/opentelemetry-specification/issues/173): Way to ignore healthcheck traces when using automatic tracer across all languages?
- issue [1060](https://github.com/open-telemetry/opentelemetry-java-instrumentation/issues/1060): Exclude URLs from Tracing
- issue [1844](https://github.com/open-telemetry/opentelemetry-specification/issues/1844): Composite Sampler

Unfortunately, some of the valuable ideas flowing at the sampling SIG meetings never got recorded at the time of their inception, but see [Sampling SIG Research Notes](https://github.com/open-telemetry/oteps/pull/213) or the comments under [OTEP 240: A Sampling Configuration proposal](https://github.com/open-telemetry/oteps/pull/240) for some examples.

## The Goal

The goal of this proposal is to help creating advanced sampling configurations using pre-defined building blocks. Let's consider the following example of sampling requirements. It is believed that many users will have requirements following a similar pattern. Most notable elements here are trace classification based on target URL, some spans requiring special handling, and putting a sanity cap on the total volume of exported spans.

Since this is a new part of the sampler specification, it is expected to be an optional component for OpenTelemetry SDKs. However, if an SDK _opts-in_, it SHOULD implement all samplers described herein.

### Example

Head-based sampling requirements:

- for root spans:
  - drop all `/healthcheck` requests
  - capture all `/checkout` requests
  - capture 25% of all other requests
- for non-root spans
  - follow the parent sampling decision
- however, capture all calls to service `/foo` (even if the trace will be incomplete)
- in any case, do not exceed 1000 spans/minute

_Note_: several proposed samplers call for calculating _unions_ of Attribute sets.
Whenever such union is constructed, in case of conflicting attribute keys, the attribute definition from the last set that uses that key takes effect. Similarly, whenever modifications of `Tracestate` are performed in sequence, in case of conflicting keys, the last modification erases the previous values.

## Proposed Samplers

A principle of operation for all new samplers is that `ShouldSample` is invoked only once, on the root of the tree formed by composite samplers.
All the logic provided by the composition of samplers is handled by calculating the threshold values through `GetSamplingIntent`, delegating the calculation downstream as necessary.

### New API

To make this approach possible, all Consistent Probability Samplers which participate in the samplers composition need to implement the following API, in addition to the standard Sampler API. We will use the term _Composable_ sampler to denote Consistent Probability Samplers which provide the new API and conform to the rules described here.
All the samplers described below are _Composable_ samplers.

#### GetSamplingIntent

This is an operation for all `Composable` samplers. Its purpose is to query the sampler about the activities it would perform had it been asked to make a sampling decision for a given span, however, without constructing the actual sampling Decision.

#### Required Arguments for GetSamplingIntent

The arguments are the same as for [`ShouldSample`](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/sdk.md#shouldsample) except for the `TraceId`.

- `Context` with parent `Span`.
- Name of the `Span` to be created.
- `SpanKind` of the `Span` to be created.
- Initial set of `Attributes` of the `Span` to be created.
- Collection of links that will be associated with the `Span` to be created.

#### Return value

The return value is a structure (`SamplingIntent`) with the following elements:

- The THRESHOLD value represented as a 14-character hexadecimal string, with value of `null` representing non-probabilistic `DROP` decision (implementations MAY use different representation, if it appears more performant or convenient),
- A function (`IsAdjustedCountReliable`) that provides a `boolean` value indicating that the adjusted count (calculated as reciprocal of the sampling probability) can be faithfully used to estimate span metrics,
- A function (`GetAttributes`) that provides a set of `Attributes` to be added to the `Span` in case of a positive final sampling decision,
- A function (`UpdateTraceState`) that, given an input `Tracestate` and sampling Decision, provides a `Tracestate` to be associated with the `Span`. The samplers SHOULD NOT add or modify the `th` value for the `ot` key within these functions. The root node of the tree of composite samplers is solely responsible for setting or clearing this value (see Constructing `SamplingResult` below).

#### Requirements for the basic samplers

The `ConsistentAlwaysOff` sampler MUST provide a `SamplingIntent` with

- The THRESHOLD value of `null` (or equivalent),
- `IsAdjustedCountReliable` returning `false`,
- `GetAttributes` returning an empty set,
- `UpdateTraceState` returning its argument, without any modifications.

The `ConsistentAlwaysOn` sampler MUST provide a `SamplingIntent` with

- The THRESHOLD value of `00000000000000` (or equivalent),
- `IsAdjustedCountReliable` returning `true`,
- `GetAttributes` returning an empty set,
- `UpdateTraceState` returning its argument, without any modifications.

The `ConsistentFixedThreshold` sampler, which is essentially `TraceIdRatioBased` sampler but implementing the `Composable` interface, MUST provide a `SamplingIntent` with

- The THRESHOLD value representing the threshold calculated according to the proposed [sampler requirements following OTEP 235](https://github.com/open-telemetry/opentelemetry-specification/pull/4166),
- `IsAdjustedCountReliable` returning `true`,
- `GetAttributes` returning an empty set,
- `UpdateTraceState` returning its argument, without any modifications.

#### Constructing `SamplingResult`

The process of constructing the final `SamplingResult` in response to a call to `ShouldSample` on the root sampler of the composite samplers tree consists of the following steps.

- The sampler gets its own `SamplingIntent`, it is a recursive process as described below (unless the sampler is a leaf),
- The sampler compares the received THRESHOLD value with the trace Randomness value to arrive at the final sampling `Decision`,
- The sampler calls the received `UpdateTraceState` function passing the parent `Tracestate` and the final sampling `Decision` to get the new `Tracestate` to be associated with the `Span` - again, in most cases this is a recursive step,
- In case of positive sampling decision:
  - the sampler calls the received `GetAttributes` function to determine the set of `Attributes` to be added to the `Span`, in most cases it will be a recursive step,
  - the sampler calls the received `IsAdjustedCountReliable` function, and in case of `true` it modifies the `th` value for the `ot` key in the `Tracestate` according to the received THRESHOLD; if the returned value is `false`, it removes the `th` value for the `ot` key from the `Tracestate`,
- In case of negative sampling decision, it removes the `th` value for the `ot` key from the `Tracestate`.

### ConsistentRuleBased

`ConsistentRuleBased` is a composite sampler which performs `Span` categorization (e.g. when sampling decision depends on `Span` attributes) and sampling.
The Spans can be grouped into separate categories, and each category can use a different Sampler.
Categorization of Spans is aided by `Predicates`.

#### Predicate

The Predicates represent logical expressions which can access `Span` `Attributes` (or anything else available when the sampling decision is to be made), and perform tests on the accessible values.
For example, one can test if the target URL for a SERVER span matches a given pattern.
`Predicate` interface allows users to create custom categories based on information that is available at the time of making the sampling decision.
To preserve integrity of consistent probability sampling, the Predicates MUST NOT depend on the parent `sampled` flag nor the lowest 56-bit of the `TraceId` (which can be representing the _randomness_ value).

##### SpanMatches

This is an operation for `Predicate`, which returns `true` if a given `Span` matches, i.e. belongs to the category described by the Predicate.

##### Required Arguments for Predicates

The arguments represent the values that are made available for `ShouldSample`.

- `Context` with parent `Span`.
- `TraceId` of the `Span` to be created.
- Name of the `Span` to be created.
- Initial set of `Attributes` of the `Span` to be created.
- Collection of links that will be associated with the `Span` to be created.

#### Required Arguments for ConsistentRuleBased

- optional `SpanKind`,
- list of pairs (`Predicate`, `Composable`)

For calculating the `SamplingIntent`, if the `Span` kind matches the specified kind, or the specified kind is not given, the sampler goes through the list in the provided order and calls `SpanMatches` on `Predicate`s passing the same arguments as received.
If a call returns `true`, the result is as returned by `GetSamplingIntent` called on the corresponding `Composable` - no other `Predicate`s are evaluated.
If the `SpanKind` does not match, or none of the calls to `SpanMatches` yield `true`, the result is obtained by calling `GetSamplingIntent` on `ConsistentAlwaysOffSampler`.

### ConsistentParentBased

The functionality of `ConsistentParentBased` sampler corresponds to the standard `ParentBased` sampler.
It takes one `Composable` sampler delegate as the argument.
The delegate is used to make sampling decisions for ROOT spans.

The behavior of `ConsistentParentBased` caters to the case where a non-probabilistic sampler was used to sample the parent span.

Upon invocation of its `GetSamplingIntent` operation the sampler checks if there's a valid parent span context. If there isn't, the sampler MUST return the result of calling `GetSamplingIntent` on the delegate.

Otherwise, the sampler attempts to extract the threshold value from the parent trace state.
The sampler MUST return a `SamplingIntent` as follows.

If the parent trace state has a valid threshold T:

- The resulting THRESHOLD value is T.
- The `IsAdjustedCountReliable` returns `true`.

If the parent trace state has no valid threshold, the sampler examines the `sampled` flag from the traceparent.

If the flag is set:

- The resulting THRESHOLD value is `00000000000000` (or equivalent).
- The `IsAdjustedCountReliable` returns `false`.

If the flag is not set:

- The resulting THRESHOLD value is `null` (or equivalent).
- The `IsAdjustedCountReliable` returns `false`.

By default, in all cases with valid parent context:

- The `GetAttributes` function returns empty set.
- The `UpdateTraceState` function returns its argument, without any modifications.

However, `ConsistentParentBased` implementations SHOULD allow users to customize added Attributes as well as modify the trace state depending on whether the parent Span is local or remote.

### ConsistentAnyOf

`ConsistentAnyOf` is a composite sampler which takes a non-empty list of `Composable` samplers (delegates) as the argument. The intention is to make a positive sampling decision if _any of_ the delegates would make a positive decision.

Upon invocation of its `GetSamplingIntent` operation, it MUST go through the whole list and invoke `GetSamplingIntent` operation on each delegate sampler, passing the same arguments as received.

`ConsistentAnyOf` sampler MUST return a `SamplingIntent` which is constructed as follows:

- If any of the delegates returned a non-`null` threshold value, the resulting threshold is the lexicographical minimum value T from the set of those non-`null` values, otherwise `null`.
- The `IsAdjustedCountReliable` returns `true`, if any of the delegates returning the threshold value equal to T returns `true` upon calling its `IsAdjustedCountReliable` function, otherwise it returns `false`.
- The `GetAttributes` function calculates the union of `Attribute` sets as returned by the calls to `GetAttributes` function for each delegate, in the declared order.
- The `UpdateTraceState` function makes a chain of calls to the `UpdateTraceState` functions as returned by the delegates, passing the received `Tracestate` as argument to subsequent calls and returning the last value received.

Each delegate sampler MUST be given a chance to participate in calculating the `SamplingIntent` as described above and MUST see the same argument values. The order of the delegate samplers does not affect the final sampling `Decision`.

### ConsistentRateLimiting

`ConsistentRateLimiting` is a composite sampler that helps control the average rate of sampled spans while allowing another sampler (the delegate) to provide sampling hints.

#### Required Arguments for ConsistentRateLimiting

- Composable (delegate)
- maximum sampling (throughput) target rate

The sampler SHOULD measure and keep the average rate of incoming spans, and therefore also of the desired ratio between the incoming span rate to the target span rate.
Upon invocation of its `GetSamplingIntent` operation, the composite sampler MUST get the `SamplingIntent` from the delegate sampler, passing the same arguments as received.

The returned `SamplingIntent` is constructed as follows.

- If using the obtained threshold value as the final threshold would entail sampling more spans than the declared target rate, the sampler SHOULD set the threshold to a value that would meet the target rate. Several algorithms can be used for threshold adjustment, no particular behavior is prescribed by the specification though.
- The `IsAdjustedCountReliable` returns the result of calling this function on the `SamplingIntent` provided by the delegate.
- The `GetAttributes` function returns the result of calling this function on the `SamplingIntent` provided by the delegate.
- The `UpdateTraceState` function returns the `Tracestate` as returned by calling `UpdateTraceState` from the delegate's `SamplingIntent`.

## Summary

### Example - sampling configuration

Going back to our [example](#example) of sampling requirements, we can now configure the head sampler to support this particular case, using an informal notation of samplers and their arguments.
First, let's express the requirements for the ROOT spans as follows.

```
S1 = ConsistentRuleBased(ROOT, {
 (http.target == /healthcheck) => ConsistentAlwaysOff,
 (http.target == /checkout) => ConsistentAlwaysOn,
 true => ConsistentFixedThreshold(0.25)
 })
```

Note: technically, `ROOT` is not a `SpanKind`, but is a special token matching all Spans with invalid parent context (i.e. the ROOT spans, regardless of their kind).

In the next step, we can build the sampler to handle non-root spans as well:

```
S2 = ConsistentParentBased(S1)
```

The special case of calling service `/foo` can now be supported by:

```
S3 = ConsistentAnyOf(S2, ConsistentRuleBased(CLIENT, {
           (http.url == /foo) => ConsistentAlwaysOn
     }))
```

Finally, the last step is to put a limit on the stream of exported spans:

```
S4 = ConsistentRateLimiting(S3, 1000)
```

This is the complete example:

```
S4 = 
ConsistentRateLimiting(
    ConsistentAnyOf(
        ConsistentParentBased(
            ConsistentRuleBased(ROOT, {
                (http.target == /healthcheck) => ConsistentAlwaysOff,
                (http.target == /checkout) => ConsistentAlwaysOn,
                true => ConsistentFixedThreshold(0.25),
            })
        ),
        ConsistentRuleBased(CLIENT, {
            (http.url == /foo) = > ConsistentAlwaysOn,
        }),
    ),
    1000,
)
```

### Limitations

Developers of `Composable` samplers should consider that the sampling Decision they declare as their intent might be different from the final sampling Decision.

### Prototyping

A prototype implementation of Composable Samplers for Java is available, see [ConsistentSampler](https://github.com/open-telemetry/opentelemetry-java-contrib/blob/main/consistent-sampling/src/main/java/io/opentelemetry/contrib/sampler/consistent56/ConsistentSampler.java) and its subclasses.

## Prior art

A number of composite samplers are already available as independent contributions
([RuleBasedRoutingSampler](https://github.com/open-telemetry/opentelemetry-java-contrib/blob/main/samplers/src/main/java/io/opentelemetry/contrib/sampler/RuleBasedRoutingSampler.java),
[Stratified Sampling](https://github.com/open-telemetry/opentelemetry-dotnet/tree/main/docs/trace/stratified-sampling-example),
LinksBasedSampler [for Java](https://github.com/open-telemetry/opentelemetry-java-contrib/blob/main/samplers/src/main/java/io/opentelemetry/contrib/sampler/LinksBasedSampler.java)
and [for DOTNET](https://github.com/open-telemetry/opentelemetry-dotnet/tree/main/docs/trace/links-based-sampler)).
Also, historically, some Span categorization was introduced by [JaegerRemoteSampler](https://www.jaegertracing.io/docs/1.54/sampling/#remote-sampling).

This proposal aims at generalizing these ideas, and at providing a bit more formal specification for the behavior of the composite samplers.
