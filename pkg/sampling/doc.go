// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// # TraceState representation
//
// A [W3CTraceState] object parses and stores the OpenTelemetry
// tracestate field and any other fields that are present in the
// W3C tracestate header, part of the [W3C tracecontext specification].
//
// An [OpenTelemetryTraceState] object parses and stores fields of
// the OpenTelemetry-specific tracestate field, including those recognized
// for probability sampling and any other fields that are present.  The
// syntax of the OpenTelemetry field is specified in [Tracestate handling].
//
// The probability sampling-specific fields used here are specified in
// [OTEP 235].  The principle, named fields are:
//
//   - T-value: The sampling rejection threshold, expresses a 56-bit
//     hexadecimal number of traces that will be rejected by sampling.
//   - R-value: The sampling randomness value can be implicit in a TraceID,
//     otherwise it is explicitly encoded as an R-value.
//
// # Low-level types
//
// The three key data types implemented in this package represent sampling
// decisions.
//
//   - [Threshold]: Represents an exact sampling probability.
//   - [Randomness]: Randomness used for sampling decisions.
//   - [Threshold.Probability]: a float64 in the range [MinSamplingProbability, 1.0].
//
// [W3C tracecontext specification]: https://www.w3.org/TR/trace-context/#tracestate-header
// [Tracestate handling]: https://opentelemetry.io/docs/specs/otel/trace/tracestate-handling/
package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
