// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator"

import (
	"encoding/binary"
	"strconv"
	"strings"

	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	"github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace/idx"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/metadata"
)

// The tracers and the Datadog Agent set the sampling tags. For example, dd-trace-go defines them in
// https://github.com/DataDog/dd-trace-go/blob/3bef7147ab8b5d11317f7ae4a0931b714b57795d/ddtrace/tracer/span.go#L1496-L1541
const (
	// The Agent's sampling rate. It is exact and on the local root.
	keyAgentPSR = "_dd.agent_psr"
	// The Knuth sampling rate, rounded to 6 decimals.
	keyKnuthRate = "_dd.p.ksr"
	// The rate of the Agent's probabilistic sampler. It is exact and on the root.
	keyProbabilisticRate = "_dd.prob_sr"
	// The rate of a tracer sampling rule, and the rate of the rate limiter applied to it. dd-trace-java sets the
	// limit to a count per second, so values above 1 are not rates.
	keyRulePSR       = "_dd.rule_psr"
	keyLimitPSR      = "_dd.limit_psr"
	keyDecisionMaker = "_dd.p.dm"
	// The tracestate that the tracer received, which it keeps as a tag of the first span of a chunk.
	keyTracestate = "tracestate"

	priorityAutoKeep = 1
	priorityUserKeep = 2

	// The multiplier of the Knuth hash of the lower 64 bits of the trace id. A trace is kept if the hash is at most
	// the rate times the largest uint64. See sampledByRate in
	// https://github.com/DataDog/dd-trace-go/blob/3bef7147ab8b5d11317f7ae4a0931b714b57795d/ddtrace/tracer/sampler.go#L95-L120
	knuthFactor = uint64(1111111111111111111)
)

// samplingMechanism is the `_dd.p.dm` tag, a negative number like -3, or 0 when there is none. See
// https://github.com/DataDog/dd-trace-go/blob/3bef7147ab8b5d11317f7ae4a0931b714b57795d/internal/samplernames/samplernames.go#L22-L53
type samplingMechanism int32

const (
	mechanismAgentRate      samplingMechanism = -1
	mechanismRemoteAutoRate samplingMechanism = -2
	mechanismLocalRule      samplingMechanism = -3
	mechanismRemoteUserRate samplingMechanism = -6
	// The Agent's probabilistic sampler hashes the trace id with a seed (FNV) instead of the Knuth factor. See
	// https://github.com/DataDog/datadog-agent/blob/feda1819a7602228b238c9913a88ff6a0cfb1783/pkg/trace/agent/agent.go#L62
	mechanismAgentProbabilistic samplingMechanism = -9
	mechanismRemoteUserRule     samplingMechanism = -11
	mechanismRemoteAdaptiveRule samplingMechanism = -12
)

// decidedByRate returns whether a rate decided that a chunk was kept. Priority 1 is a keep by rate. Priority 2 is a
// forced keep, such as a manual keep or AppSec, unless the mechanism is a rate or a rule. Other priorities are drops.
func decidedByRate(priority float64, mechanism samplingMechanism) bool {
	switch priority {
	case priorityAutoKeep:
		return true
	case priorityUserKeep:
		switch mechanism {
		case mechanismAgentRate, mechanismRemoteAutoRate, mechanismLocalRule, mechanismRemoteUserRate,
			mechanismAgentProbabilistic, mechanismRemoteUserRule, mechanismRemoteAdaptiveRule:
			return true
		}
	}
	return false
}

// parseMechanism returns 0 when s is not a negative number.
func parseMechanism(s string) samplingMechanism {
	m, err := strconv.ParseInt(s, 10, 32)
	if err != nil || m >= 0 {
		return 0
	}
	return samplingMechanism(m)
}

// chunkMechanism returns the mechanism from the tags of the chunk, or else from the first span that has one.
func chunkMechanism(chunk *pb.TraceChunk) samplingMechanism {
	if m := parseMechanism(chunk.GetTags()[keyDecisionMaker]); m != 0 {
		return m
	}
	for _, span := range chunk.GetSpans() {
		if m := parseMechanism(span.Meta[keyDecisionMaker]); m != 0 {
			return m
		}
	}
	return 0
}

// chunkMechanismV1 returns the mechanism of a v1 chunk, which stores it without the minus sign.
func chunkMechanismV1(chunk *idx.InternalTraceChunk) samplingMechanism {
	return -samplingMechanism(chunk.SamplingMechanism())
}

type rateLookup func(key string) (float64, bool)

var rateKeys = [...]string{keyAgentPSR, keyProbabilisticRate, keyKnuthRate, keyRulePSR, keyLimitPSR}

// rateTags holds the first valid rate, in (0, 1], of each sampling tag. Tracers spread the tags over the spans of a
// chunk, so the rate of a chunk is built from all of them.
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

// probability returns the probability that the chunk was kept with, and whether a Knuth hash of the trace id alone
// decided it. `_dd.p.ksr` and `_dd.rule_psr` are the rate of the rule without the rate limiter, which also rejects
// traces, so they are multiplied by `_dd.limit_psr`. Decisions of the Agent's probabilistic sampler or of the rate
// limiter do not follow the Knuth hash.
func (t rateTags) probability(mechanism samplingMechanism) (probability float64, knuth, ok bool) {
	if mechanism == mechanismAgentProbabilistic {
		probability, ok = t[keyProbabilisticRate]
		return probability, false, ok
	}
	if p, ok := t[keyAgentPSR]; ok {
		return p, true, true
	}
	if p, ok := t[keyProbabilisticRate]; ok {
		return p, false, true
	}
	limit, hasLimit := t[keyLimitPSR]
	if !hasLimit {
		limit = 1
	}
	if p, ok := t[keyKnuthRate]; ok {
		return p * limit, limit == 1, true
	}
	if p, ok := t[keyRulePSR]; ok {
		return p * limit, false, true
	}
	return 0, false, false
}

func parseRate(s string) (float64, bool) {
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

// inboundTraceState parses the tracestate that the tracer received, which it keeps as a tag. An invalid one is dropped.
func inboundTraceState(logger *zap.Logger, value string) sampling.W3CTraceState {
	w3c, err := sampling.NewW3CTraceState(value)
	if err != nil {
		logger.Debug("cannot parse the inbound tracestate", zap.Error(err))
		return sampling.W3CTraceState{}
	}
	return w3c
}

// chunkTraceState returns the tracestate for the spans of a legacy format chunk. It is the tracestate that the tracer
// received, with the sampling rate of Datadog as the threshold, unless the tracestate has a threshold already. The tags
// are span metrics or meta, or tags of the chunk.
func chunkTraceState(logger *zap.Logger, chunk *pb.TraceChunk, priority float64) string {
	if !metadata.ReceiverDatadogEnableSamplingThresholdFeatureGate.IsEnabled() || len(chunk.GetSpans()) == 0 {
		return ""
	}
	inbound := chunk.GetTags()[keyTracestate]
	for i := 0; inbound == "" && i < len(chunk.GetSpans()); i++ {
		inbound = chunk.GetSpans()[i].Meta[keyTracestate]
	}
	w3c := inboundTraceState(logger, inbound)
	if mechanism := chunkMechanism(chunk); !hasThreshold(&w3c) && decidedByRate(priority, mechanism) {
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
		setSamplingRate(logger, &w3c, tags, mechanism, chunk.GetSpans()[0].TraceID)
	}
	return serializeTraceState(logger, &w3c)
}

// chunkTraceStateV1 returns the tracestate for the spans of a v1 chunk. It is the tracestate that the tracer received,
// with the sampling rate of Datadog as the threshold, unless the tracestate has a threshold already. The tags are
// attributes of the chunk or of its spans, as numbers or strings.
func chunkTraceStateV1(logger *zap.Logger, chunk *idx.InternalTraceChunk) string {
	if !metadata.ReceiverDatadogEnableSamplingThresholdFeatureGate.IsEnabled() || len(chunk.Spans) == 0 || len(chunk.TraceID) != 16 {
		return ""
	}
	inbound, _ := chunk.GetAttributeAsString(keyTracestate)
	for i := 0; inbound == "" && i < len(chunk.Spans); i++ {
		inbound, _ = chunk.Spans[i].GetAttributeAsString(keyTracestate)
	}
	w3c := inboundTraceState(logger, inbound)
	if mechanism := chunkMechanismV1(chunk); !hasThreshold(&w3c) && decidedByRate(float64(chunk.Priority), mechanism) {
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
		setSamplingRate(logger, &w3c, tags, mechanism, binary.BigEndian.Uint64(chunk.TraceID[8:]))
	}
	return serializeTraceState(logger, &w3c)
}

// hasThreshold returns whether an upstream sampler set a threshold, which is the combined probability of all sampling
// stages, so Datadog's rate does not apply on top of it.
func hasThreshold(w3c *sampling.W3CTraceState) bool {
	_, ok := w3c.OTelValue().TValueThreshold()
	return ok
}

// setSamplingRate sets the `th` threshold of the chunk's rate. When a Knuth hash decided and the tracestate has no
// randomness value yet, it also sets the `rv` that gives the same decision, so that `rv >= th` for the kept spans.
func setSamplingRate(logger *zap.Logger, w3c *sampling.W3CTraceState, tags rateTags, mechanism samplingMechanism, traceID uint64) {
	rate, knuth, ok := tags.probability(mechanism)
	if !ok {
		return
	}
	threshold, err := sampling.ProbabilityToThreshold(rate)
	if err != nil {
		logger.Debug("cannot convert the sampling rate to a threshold", zap.Float64("rate", rate), zap.Error(err))
		return
	}
	otel := w3c.OTelValue()
	var rnd sampling.Randomness
	setRandomness := knuth && otel.RValue() == ""
	if setRandomness {
		// rv is the inverse of the hash h = traceID * knuthFactor, truncated to 56 bits. Truncation and rounded rate
		// tags can leave it just below th, so it is raised to th.
		if rnd, err = sampling.UnsignedToRandomness(max((^(traceID * knuthFactor))>>8, threshold.Unsigned())); err != nil {
			logger.Debug("cannot set the randomness value", zap.Error(err))
			return
		}
	}
	if err := otel.UpdateTValueWithSampling(threshold); err != nil {
		logger.Debug("cannot set the sampling threshold", zap.Error(err))
		return
	}
	if setRandomness {
		otel.SetRValue(rnd)
	}
}

func serializeTraceState(logger *zap.Logger, w3c *sampling.W3CTraceState) string {
	var sb strings.Builder
	if err := w3c.Serialize(&sb); err != nil {
		logger.Debug("cannot serialize the tracestate", zap.Error(err))
		return ""
	}
	return sb.String()
}
