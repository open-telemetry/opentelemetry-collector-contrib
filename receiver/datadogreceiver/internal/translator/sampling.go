// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator"

import (
	"strconv"
	"strings"

	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	"github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace/idx"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

const (
	// The sampling rate of the Datadog Agent's priority sampler. It is exact, and only on the deciding local root.
	keyAgentPSR = "_dd.agent_psr"
	// The Knuth sampling rate, with 6 decimals, on the first span of every chunk.
	keyKnuthRate = "_dd.p.ksr"
	// The rate of the Datadog Agent's own probabilistic sampler, as a fraction. It is exact, and only on the root.
	keyProbabilisticRate = "_dd.prob_sr"
	// The rate of the tracer's sampling rule, and the effective rate of the limiter that applies to it. dd-trace-java
	// sets the limit tag to the configured limit per second instead of a rate. That is above 1, so it is ignored.
	keyRulePSR  = "_dd.rule_psr"
	keyLimitPSR = "_dd.limit_psr"
	// The decision maker: the mechanism that made the sampling decision, written as a negative number like "-3".
	keyDecisionMaker = "_dd.p.dm"

	priorityAutoKeep = 1
	priorityUserKeep = 2
)

// decidedByRate returns whether a rate decided that Datadog kept a chunk, so that the rate is the probability that it
// was kept with. Priority 1 is the keep by rate. Priority 2 is a keep that a user forced, which is not decided by a
// rate, except for the sampling rules of a tracer, which also keep with priority 2. The decision maker tells these
// apart. A drop (0 or less, for example a chunk that was rescued because it has an error) was not decided by a rate.
func decidedByRate(priority float64, mechanism func() (uint32, bool)) bool {
	switch priority {
	case priorityAutoKeep:
		return true
	case priorityUserKeep:
		m, ok := mechanism()
		return ok && rateMechanisms[m]
	}
	return false
}

// rateMechanisms are the decision makers that keep a trace with a rate: the rate of the Agent, the rules and rates of
// the tracer and its remote configuration, and the probabilistic sampler of the Agent. The others, like a manual keep
// or AppSec, force a keep.
var rateMechanisms = map[uint32]bool{
	1:  true, // The rate that the Agent sent to the tracer.
	2:  true, // A remote automatic rate.
	3:  true, // A local sampling rule or rate.
	6:  true, // A remote user rate.
	9:  true, // The probabilistic sampler of the Agent.
	11: true, // A remote user rule.
	12: true, // A remote adaptive rule.
}

// parseMechanism parses a decision maker tag like "-3".
func parseMechanism(s string) (uint32, bool) {
	m, err := strconv.ParseUint(strings.TrimPrefix(s, "-"), 10, 32)
	return uint32(m), err == nil
}

// chunkMechanism returns the decision maker of a chunk in the legacy formats. It is a tag of the first span of the
// chunk or, in the v0.7 format, of the chunk.
func chunkMechanism(chunk *pb.TraceChunk) (uint32, bool) {
	if m, ok := parseMechanism(chunk.GetTags()[keyDecisionMaker]); ok {
		return m, true
	}
	for _, span := range chunk.GetSpans() {
		if m, ok := parseMechanism(span.Meta[keyDecisionMaker]); ok {
			return m, true
		}
	}
	return 0, false
}

// rateLookup returns the numeric value of a Datadog sampling tag.
type rateLookup func(key string) (float64, bool)

// rateKeys are the tags that tell the sampling rate of a chunk.
var rateKeys = [...]string{keyAgentPSR, keyProbabilisticRate, keyKnuthRate, keyRulePSR, keyLimitPSR}

// rateTags are the sampling tags of a chunk, with the first valid probability (in (0, 1]) of each. The tags are spread
// over the spans of a chunk, and a tracer can put them on different spans, like the Knuth rate on the first span of
// a chunk and the rule and limiter rates on the root. So a chunk has one rate, built from all of them.
type rateTags map[string]float64

func (t rateTags) collect(lookup rateLookup) {
	for _, key := range rateKeys {
		if _, ok := t[key]; ok {
			continue
		}
		if p, ok := lookup(key); ok && p > 0 && p <= 1 {
			t[key] = p
		}
	}
}

// probability returns the probability that Datadog kept the chunk with. It prefers the exact `_dd.agent_psr` or
// `_dd.prob_sr`, then `_dd.p.ksr`, then `_dd.rule_psr`. The Knuth rate and the rule rate are the rate of the rule
// alone, and the limiter of the rule also rejects a part of the traces that the rule sampled, so they are multiplied
// by the rate of the limiter (`_dd.limit_psr`) when there is one.
func (t rateTags) probability() (float64, bool) {
	for _, key := range [...]string{keyAgentPSR, keyProbabilisticRate} {
		if p, ok := t[key]; ok {
			return p, true
		}
	}
	limit, ok := t[keyLimitPSR]
	if !ok {
		limit = 1
	}
	for _, key := range [...]string{keyKnuthRate, keyRulePSR} {
		if p, ok := t[key]; ok {
			return p * limit, true
		}
	}
	return 0, false
}

// chunkSamplingRate returns the sampling rate of a chunk in the legacy formats, which send the tags as span metrics,
// span meta or, in the v0.7 format, as tags of the chunk.
func chunkSamplingRate(chunk *pb.TraceChunk) (float64, bool) {
	tags := rateTags{}
	tags.collect(func(key string) (float64, bool) { return parseRate(chunk.GetTags()[key]) })
	for _, span := range chunk.GetSpans() {
		tags.collect(func(key string) (float64, bool) {
			if v, ok := span.Metrics[key]; ok {
				return v, true
			}
			return parseRate(span.Meta[key])
		})
	}
	return tags.probability()
}

// chunkSamplingRateV1 returns the sampling rate of a chunk in the v1 format, from the attributes of the chunk and of
// its spans. An attribute can be a number or a string, like the tags of the legacy formats.
func chunkSamplingRateV1(chunk *idx.InternalTraceChunk) (float64, bool) {
	tags := rateTags{}
	tags.collect(func(key string) (float64, bool) {
		v, ok := chunk.GetAttributeAsString(key)
		if !ok {
			return 0, false
		}
		return parseRate(v)
	})
	for _, span := range chunk.Spans {
		tags.collect(span.GetAttributeAsFloat64)
	}
	return tags.probability()
}

func parseRate(s string) (float64, bool) {
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

// setSamplingThreshold records the rate that Datadog sampled a span with as the threshold of the W3C tracestate of
// the span, so that components like spanmetrics can extrapolate counts from it. The other members of the tracestate
// are kept.
//
// The threshold of a tracestate is the combined probability of all earlier sampling stages, and a later stage can only
// raise it. Datadog's sampling is a later stage, so a threshold that is already there is multiplied with the rate, like
// a proportional downstream sampler does. A threshold that is inconsistent, or that cannot be represented, is erased,
// because the probability is then unknown. See https://opentelemetry.io/docs/specs/otel/trace/tracestate-probability-sampling/.
func setSamplingThreshold(logger *zap.Logger, span ptrace.Span, rate float64) {
	w3c, err := sampling.NewW3CTraceState(span.TraceState().AsRaw())
	if err != nil {
		logger.Debug("cannot parse the tracestate of the span", zap.Error(err))
		return
	}
	otel := w3c.OTelValue()

	probability := rate
	existing, hasExisting := otel.TValueThreshold()
	if hasExisting {
		// The span was kept, so an explicit randomness has to be at least the threshold.
		if rnd, ok := otel.RValueRandomness(); ok && !existing.ShouldSample(rnd) {
			otel.ClearTValue()
			writeTraceState(logger, span, &w3c)
			return
		}
		if rate == 1 {
			// Datadog kept every trace, so the exact earlier threshold stays.
			return
		}
		probability = rate * existing.Probability()
	}

	threshold, err := sampling.ProbabilityToThreshold(probability)
	if err != nil {
		// The probability is below the smallest one that a threshold can represent.
		logger.Debug("cannot convert the sampling rate to a threshold", zap.Float64("rate", probability), zap.Error(err))
		if hasExisting {
			otel.ClearTValue()
			writeTraceState(logger, span, &w3c)
		}
		return
	}
	if err := otel.UpdateTValueWithSampling(threshold); err != nil {
		// Rounding made the threshold lower than the earlier one, which is more precise, so that one stays.
		return
	}
	writeTraceState(logger, span, &w3c)
}

func writeTraceState(logger *zap.Logger, span ptrace.Span, w3c *sampling.W3CTraceState) {
	var sb strings.Builder
	if err := w3c.Serialize(&sb); err != nil {
		logger.Debug("cannot serialize the tracestate of the span", zap.Error(err))
		return
	}
	span.TraceState().FromRaw(sb.String())
}
