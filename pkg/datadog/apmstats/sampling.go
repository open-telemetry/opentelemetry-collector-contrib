// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apmstats // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/apmstats"

import (
	"strconv"

	traceutilotel "github.com/DataDog/datadog-agent/pkg/trace/otel/traceutil"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

// keySamplingRateGlobal mirrors the unexported constant in
// github.com/DataDog/datadog-agent/pkg/trace/stats (weight.go).
// Must stay in sync: the Concentrator reads this key to compute span weight.
const keySamplingRateGlobal = "_sample_rate"

// pValueNotSampled is the reserved p-value sentinel meaning "not sampled"
// in the consistent-probability sampling encoding (p:63 has no probability).
const pValueNotSampled = 63

// rawTracestatesBySpanID returns a map of DD span ID (uint64) to the raw W3C
// tracestate string for each OTel span that has a non-empty tracestate. No
// parsing happens here; the tracestate is only parsed later for spans that
// are actually chunk roots (see samplingProbFromTracestate).
func rawTracestatesBySpanID(traces ptrace.Traces) map[uint64]string {
	result := make(map[uint64]string)
	rss := traces.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		sss := rss.At(i).ScopeSpans()
		for j := 0; j < sss.Len(); j++ {
			spans := sss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				raw := span.TraceState().AsRaw()
				if raw == "" {
					continue
				}
				result[traceutilotel.OTelSpanIDToUint64(span.SpanID())] = raw
			}
		}
	}
	return result
}

// samplingProbFromSpan extracts the sampling probability from a span's W3C
// tracestate. Returns (probability, true) on success, (0, false) otherwise.
//
// Two encodings are supported:
//   - th (threshold): OTel collector-contrib pkg/sampling samplers.
//     e.g. "ot=th:8" → probability 0.5
//   - p (power-of-two): go.opentelemetry.io/contrib/samplers/probability/consistent.
//     e.g. "ot=p:1;r:1" → probability 2^-1 = 0.5
func samplingProbFromSpan(span ptrace.Span, logger *zap.Logger) (float64, bool) {
	return samplingProbFromTracestate(span.TraceState().AsRaw(), logger)
}

// samplingProbFromTracestate extracts the sampling probability from a raw W3C
// tracestate string. Returns (probability, true) on success, (0, false)
// otherwise. See samplingProbFromSpan for the supported encodings.
func samplingProbFromTracestate(raw string, logger *zap.Logger) (float64, bool) {
	if raw == "" {
		return 0, false
	}
	w3c, err := sampling.NewW3CTraceState(raw)
	if err != nil {
		// Malformed tracestate — log at debug so misconfigured tracers are
		// diagnosable without being noisy in healthy pipelines.
		if logger != nil {
			logger.Debug("Failed to parse W3C tracestate for sampling probability",
				zap.String("tracestate", raw), zap.Error(err))
		}
		return 0, false
	}
	otel := w3c.OTelValue()

	// th encoding: used by the OTel collector-contrib pkg/sampling samplers.
	if th, ok := otel.TValueThreshold(); ok {
		return th.Probability(), true
	}

	// p encoding: used by go.opentelemetry.io/contrib/samplers/probability/consistent.
	// p:N means sampling probability = 2^-N (e.g. p:1 → 0.5, p:4 → 1/16).
	// Valid range is [0, 62]; p:63 is the reserved "not sampled" sentinel and
	// carries no meaningful probability, so we skip it.
	for _, kv := range otel.ExtraValues() {
		if kv.Key != "p" {
			continue
		}
		pVal, err := strconv.ParseUint(kv.Value, 10, 64)
		if err != nil {
			break
		}
		if pVal >= pValueNotSampled {
			// Sentinel value meaning "not sampled"; no probability to extract.
			break
		}
		if pVal == 0 {
			return 1.0, true
		}
		return 1.0 / float64(uint64(1)<<pVal), true
	}

	return 0, false
}
