// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// ### TraceState handling types
//
// The TraceState handling routines in this package preserve unrecognized
// key values which may be present in the OpenTelemetry `tracestate` header.

// - `Threshold`: Represents an exact sampling probability.
// - `Randomness`: Randomness used for sampling decisions.
// - `Probability`: a float64 in the range `[MinSamplingProbability, 1.0]`.

// #### W3C TraceState

// A W3C `tracestate` header can be parsed to extract sampling parameters
// and pass through unrecognized fields.

// - `NewW3CTraceState`: Parse a W3C `tracestate` header with syntax like
//   `k1=v1,v2=v2`.
// - `W3CTraceState.Serialize`: After modifying a TraceState with a
//   sampling update, this produces a new string encoding to be placed
//   back into the span.

// The `W3CTraceState` accessor methods are:

// - `W3CTraceState.HasOTelValue`: Returns true when the `tracestate`
//   contains an `ot` field representing the OpenTelemetry trace state.
//   Callers may opt to take a fast path when the OpenTelemetry
//   tracestate is empty.
// - `W3CTraceState.OTelValue`: Returns a mutable `*OpenTelemetryTraceState`
//   belonging to this object.

// `W3CTraceState` objects also support the common TraceState members.

// #### OpenTelemetry TraceState

// An OpenTelemetry `tracestate` field is extracted using the `ot` field
// of the W3C TraceState and is parsed to extract sampling parameters and
// pass through unrecognized fields.

// - `NewOpenTelemetryTraceState`: Parse an OpenTelemetry `tracestate` header with
//   syntax like `k1:v1;v2:v2`.
// - `OpenTelemetryTraceState.Serialize`: After modifying a TraceState with a
//   sampling update, this produces a new `ot` value.

// There are `OpenTelemetryTraceState` accessor methods associated with `Threshold` and
// `Randomness` discussed below.  `OpenTelemetryTraceState` objects also support
// the common TraceState members.

// #### Common TraceState members

// Both W3C and OpenTelemety TraceState values allow additional,
// syntactically valid fields to be stored and re-serialized, that
// effectively pass through samplers as a result.

// - `HasAnyValue`: Returns true when the `tracestate` is not empty.
//   Callers may opt to take a fast path when a tracestate is empty.
// - `HasExtraValues`: Returns true when there are unrecognized values
//   contained in a TraceState for manual inspection.
// - `ExtraValues`: Returns a slice of `KV` corresponding with additional
//   values.  This may be by callers to inspect arbitrary fields, such as
//   the legacy `p` value representing power-of-two sampling probability.
// - `KV`: Represents unrecognized key-value pairs from in a TraceState
//   object.

// #### Sampling Threshold

// The `Threshold` type represents a parsed T-value.  Samplers that use a
// fixed probability can store these to make `ShouldSample` an
// inexpensive operation.  Samplers that use dynamic probability can
// compute them on the fly.  The constructors are:

// - `TValueToThreshold`: Parse an encoded `Threshold` from an encoded
//   OpenTelemetry TraceState `tv` field.
// - `ProbabilityToThreshold`: Translate a `float64` probability into a
//   `Threshold` with up to 14 hex-digits of precision.
// - `ProbabilityToThresholdWithPrecision`: Translate a `float64`
//   probability into a `Threshold` with a limited number of hex-digits
//   of precision.

// The `Threshold` methods are:

// - `Threshold.TValue`: Encode the T-value corresponding with `Threshold`.
// - `Threshold.ShouldSample`: Make a sampling decision for a given randomness object.
// - `Threshold.Probability`: Convert a `Threshold` to an exact probability.
// - `ThresholdGreater` and `ThresholdLessThan`: Threshold objects are
//   inversely proportional with probability (i.e., smaller threshold
//   values indicate larger sampling probabilities).  These comparison
//   functions allow a sampler to detect when sampling will be
//   inconsistent or have no change, depending on the application.  If
//   two Thresholds are neither less-than or greater to each other, they
//   are necessarily equal.

// The `OpenTelemetryTraceState` accessor methods are:

// - `OpenTelemetryTraceState.HasTValue`: Returns true when the tracestate contains a T-value,
//   indicating that `TValueThreshold` is valid.
// - `OpenTelemetryTraceState.SetTValue`: Updates the T-value, unconditioanlly modifying
//   information about probability sampling.
// - `OpenTelemetryTraceState.ClearTValue`: Allows the sampler to clear an inconsistent T-value.
// - `OpenTelemetryTraceState.TValueThreshold`: This represents a sampling threshold, primarily
//   supports the `ShouldSample(Randomness)` interface.
// - `OpenTelemetryTraceState.UpdateTValueWithSampling`: Performs a logically-consistent sampling
//   update, which modifies T-value but does not permit raising the sampling
//   probability.  When successful, this results in an change in `AdjustedCount()`.
// - `OpenTelemetryTraceState.AdjustedCount`: If T-value is present, this returns the inverse
//   sampling probability, which should be interpreted as an effective
//   count for the item.  If T-value is not present, this returns 0.

// #### Sampling Randomness

// The `Randomness` type represents a parsed R-value.  Samplers may
// interpret randomness from two sources.  The constructors are:

// - `RValueToRandomness`: Parse an encoded Randomness value from the
//   OpenTelemetry TraceState `rv` field.  This should be used when the
//   TraceState contains an `rv` field.
// - `TraceIDToRandomness`: Extract a Randomness value from a TraceID.
//   This should be used to make a sampling decision when there is
//   otherwise no randomness available.

// The `Randomness` methods are:

// - `Randomness.RValue`: Encode a `rv` field value from `Randomness`.

// The `OpenTelemetryTraceState` accessor methods are:

// - `OpenTelemetryTraceState.RValueRandomness`: Returns the pre-parsed `Randomness` value
//   corresponding with the R-value present in the TraceState.  When this
//   field is present, it will be used instead of TraceID randomness.
// - `OpenTelemetryTraceState.HasRValue`: Returns true when the tracestate contains an explicit
//   R-value, in which case `RValueRandomness()` returns a valid result.
// - `OpenTelemetryTraceState.SetRValue`: Modifies the TraceState R-value, which allows a sampler
//   to make randomness explicit.
// - `OpenTelemetryTraceState.ClearRValue`: Allows the sampler to clear an inconsistent R-value.

// #### Error values

// - `ErrTraceStateSize`: This is returned when parsing a TraceState with an over-size-limit field.
// - `ErrTValueEmpty`: Returned when T-value has size 0.
// - `ErrTValueSize`: Returned when T-value has size above 14 hex digits.
// - `ErrRValueSize`: Returned when R-value has size not equal to 14 hex digits.
// - `ErrInconsistentSampling`: Indicates an attempt to raise effective sampling probability.
// - `ErrProbabilityRange`: Sampling probability greater than 100% or less than `MinSamplingProbability`.
// - `ErrPrecisionUnderflow`: Sampling precision is too low for the stated probability.

// #### Exported values and constants

// - `AlwaysSampleThreshold`: Represents 100% sampling, which is T-value 0.
// - `MaxAdjustedCount`: Equals 2 to the power 56.
// - `MinSamplingProbability`: Equals `1/MaxAdjustedCount` (i.e., `0x1p-56`).
// - `NumHexDigits`: Equals 14, the number of hex digits required to
//   encode 56 bits. This is the maximum supported precision, the maximum
//   length of a T-value, and the expected length of an R-value.

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"

// Note that the following does not work for non-component packages:
//
//	//go:generate mdatagen metadata.yaml
