// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"
	"hash/fnv"
	"math"
	"math/big"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

const (
	defaultHashSalt = "default-hash-seed"
)

type probabilisticSampler struct {
	logger    *zap.Logger
	threshold uint64
	hashSalt  string

	useTraceState       bool
	tracestateThreshold sampling.Threshold
}

var (
	_ samplingpolicy.Evaluator          = (*probabilisticSampler)(nil)
	_ samplingpolicy.ThresholdEvaluator = (*probabilisticSampler)(nil)
)

// NewProbabilisticSampler creates a policy evaluator that samples a percentage of
// traces.
func NewProbabilisticSampler(settings component.TelemetrySettings, hashSalt string, samplingPercentage float64) samplingpolicy.Evaluator {
	if hashSalt == "" {
		hashSalt = defaultHashSalt
	}

	ratio := samplingPercentage / 100
	otelThreshold, err := sampling.ProbabilityToThreshold(ratio)
	if err != nil {
		// ProbabilityToThreshold accepts [MinSamplingProbability, 1].
		// Anything outside collapses to the nearest extreme: ratios at
		// or above 1 always sample, ratios below MinSamplingProbability
		// (including 0 and negatives) never sample.
		if ratio >= 1 {
			otelThreshold = sampling.AlwaysSampleThreshold
		} else {
			otelThreshold = sampling.NeverSampleThreshold
		}
	}

	return &probabilisticSampler{
		logger: settings.Logger,
		// calculate threshold once
		threshold:           calculateThreshold(samplingPercentage / 100),
		hashSalt:            hashSalt,
		useTraceState:       metadata.ProcessorTailsamplingprocessorUsetracestateFeatureGate.IsEnabled(),
		tracestateThreshold: otelThreshold,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (s *probabilisticSampler) Evaluate(ctx context.Context, traceID pcommon.TraceID, trace *samplingpolicy.TraceData) (samplingpolicy.Decision, error) {
	d, _, err := s.EvaluateWithThreshold(ctx, traceID, trace)
	return d, err
}

// EvaluateWithThreshold makes the sampling decision and reports the
// effective OpenTelemetry threshold the policy would advertise on
// outgoing tracestate.
//
// When the usetracestate feature gate is enabled and the trace
// carries OpenTelemetry sampling information in its W3C tracestate
// (any "ot" section with rv and/or th), the decision uses an
// explicit "rv" if present, otherwise it derives randomness from the
// TraceID per the W3C Trace Context spec, and compares that against
// the configured threshold. Otherwise it falls back to the legacy
// FNV trace ID hash.
func (s *probabilisticSampler) EvaluateWithThreshold(_ context.Context, traceID pcommon.TraceID, trace *samplingpolicy.TraceData) (samplingpolicy.Decision, sampling.Threshold, error) {
	s.logger.Debug("Evaluating spans in probabilistic filter")

	if s.useTraceState {
		if scan := scanOTelTracestate(trace.ReceivedBatches); scan.hasSamplingInfo {
			rnd := scan.randomness
			if !scan.hasRandomness {
				rnd = sampling.TraceIDToRandomness(traceID)
			}
			if s.tracestateThreshold.ShouldSample(rnd) {
				return samplingpolicy.Sampled, s.tracestateThreshold, nil
			}
			return samplingpolicy.NotSampled, s.tracestateThreshold, nil
		}
	}

	if hashTraceID(s.hashSalt, traceID[:]) <= s.threshold {
		// Report the configured OTel-equivalent threshold so
		// downstream samplers can compute adjusted counts.
		return samplingpolicy.Sampled, s.tracestateThreshold, nil
	}

	return samplingpolicy.NotSampled, s.tracestateThreshold, nil
}

func (*probabilisticSampler) IsStateful() bool {
	return false
}

type tracestateScan struct {
	// randomness is the first explicit `rv` found across any span's
	// tracestate "ot" section (spec says all spans of a trace
	// should share the same rv).
	randomness    sampling.Randomness
	hasRandomness bool
	// hasSamplingInfo is true if any span carries OTel sampling
	// information (rv and/or th). When false, the sampler falls
	// back to the legacy FNV trace ID hash.
	hasSamplingInfo bool
}

// scanOTelTracestate finds the first OTel `rv` across the trace and
// notes whether any span carries OTel sampling info at all (rv or
// th). It short-circuits as soon as both are known.
func scanOTelTracestate(td ptrace.Traces) tracestateScan {
	var result tracestateScan
	for _, rs := range td.ResourceSpans().All() {
		for _, ss := range rs.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				ts, err := sampling.NewW3CTraceState(span.TraceState().AsRaw())
				if err != nil {
					continue
				}
				otts := ts.OTelValue()
				if !result.hasRandomness {
					if rv, ok := otts.RValueRandomness(); ok {
						result.randomness = rv
						result.hasRandomness = true
						result.hasSamplingInfo = true
					}
				}
				if !result.hasSamplingInfo {
					if _, ok := otts.TValueThreshold(); ok {
						result.hasSamplingInfo = true
					}
				}
				if result.hasRandomness {
					return result
				}
			}
		}
	}
	return result
}

// calculateThreshold converts a ratio into a value between 0 and MaxUint64
func calculateThreshold(ratio float64) uint64 {
	// Use big.Float and big.Int to calculate threshold because directly convert
	// math.MaxUint64 to float64 will cause digits/bits to be cut off if the converted value
	// doesn't fit into bits that are used to store digits for float64 in Golang
	boundary := new(big.Float).SetInt(new(big.Int).SetUint64(math.MaxUint64))
	res, _ := boundary.Mul(boundary, big.NewFloat(ratio)).Uint64()
	return res
}

// hashTraceID creates a hash using the FNV-1a algorithm.
func hashTraceID(salt string, b []byte) uint64 {
	hasher := fnv.New64a()
	// the implementation fnv.Write() never returns an error, see hash/fnv/fnv.go
	_, _ = hasher.Write([]byte(salt))
	_, _ = hasher.Write(b)
	return hasher.Sum64()
}
