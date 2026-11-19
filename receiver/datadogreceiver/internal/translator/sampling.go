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

// The sampling tags below are set by the tracers and the Datadog Agent. For example, dd-trace-go defines them in
// https://github.com/DataDog/dd-trace-go/blob/3bef7147ab8b5d11317f7ae4a0931b714b57795d/ddtrace/tracer/span.go#L1496-L1541
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

	// The factor of the Knuth hash of the lower 64 bits of the trace id, which every tracer uses to decide if it keeps a
	// trace: keep if the hash is at most the rate times the largest uint64. See sampledByRate in
	// https://github.com/DataDog/dd-trace-go/blob/3bef7147ab8b5d11317f7ae4a0931b714b57795d/ddtrace/tracer/sampler.go#L95-L120
	knuthFactor = uint64(1111111111111111111)
)

// samplingMechanism is the decision maker of a sampling decision, the `_dd.p.dm` tag. It is written as a negative
// number, like "-3". 0 is no decision maker. The mechanisms that keep a trace with a rate are listed in
// https://github.com/DataDog/dd-trace-go/blob/3bef7147ab8b5d11317f7ae4a0931b714b57795d/internal/samplernames/samplernames.go#L22-L53.
// The others, like a manual keep (-4) or AppSec (-5), force a keep.
type samplingMechanism int32

const (
	// The rate that the Agent sent to the tracer.
	mechanismAgentRate samplingMechanism = -1
	// A remote automatic rate.
	mechanismRemoteAutoRate samplingMechanism = -2
	// A local sampling rule or rate.
	mechanismLocalRule samplingMechanism = -3
	// A remote user rate.
	mechanismRemoteUserRate samplingMechanism = -6
	// The probabilistic sampler of the Agent, which hashes the trace id with a seed (FNV) instead of with the Knuth
	// factor. See https://github.com/DataDog/datadog-agent/blob/feda1819a7602228b238c9913a88ff6a0cfb1783/pkg/trace/agent/agent.go#L62
	mechanismAgentProbabilistic samplingMechanism = -9
	// A remote user rule.
	mechanismRemoteUserRule samplingMechanism = -11
	// A remote adaptive rule.
	mechanismRemoteAdaptiveRule samplingMechanism = -12
)

// decidedByRate returns whether a rate decided that Datadog kept a chunk, so that the rate is the probability that it
// was kept with. Priority 1 is the keep by rate. Priority 2 is a keep that a user forced, which is not decided by a
// rate, except for the sampling rules of a tracer, which also keep with priority 2. The decision maker tells these
// apart. A drop (0 or less, for example a chunk that was rescued because it has an error) was not decided by a rate.
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

// parseMechanism parses a decision maker tag like "-3". It returns 0 when the tag is not a negative number.
func parseMechanism(s string) samplingMechanism {
	m, err := strconv.ParseInt(s, 10, 32)
	if err != nil || m >= 0 {
		return 0
	}
	return samplingMechanism(m)
}

// chunkMechanism returns the decision maker of a chunk in the legacy formats. It is a tag of the first span of the
// chunk or, in the v0.7 format, of the chunk.
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

// chunkMechanismV1 returns the decision maker of a chunk in the v1 format, which has it without the minus sign.
func chunkMechanismV1(chunk *idx.InternalTraceChunk) samplingMechanism {
	return -samplingMechanism(chunk.SamplingMechanism())
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

// probability returns the probability that Datadog kept the chunk with, and whether a Knuth hash of the trace id alone
// made the decision. It prefers the exact `_dd.agent_psr` or `_dd.prob_sr`, then `_dd.p.ksr`, then `_dd.rule_psr`. The
// Knuth rate and the rule rate are the rate of the rule alone, and the limiter of the rule also rejects a part of the
// traces that the rule sampled, so they are multiplied by the rate of the limiter (`_dd.limit_psr`) when there is one.
// A decision that the limiter influenced is not one of the Knuth hash alone, and neither is the decision of the
// probabilistic sampler of the Agent, which overrides the one of the tracer.
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

// chunkTraceState returns the tracestate for the spans of a chunk in the legacy formats, which send the tags as span
// metrics, span meta or, in the v0.7 format, as tags of the chunk. It is empty when the chunk has no sampling rate.
func chunkTraceState(logger *zap.Logger, chunk *pb.TraceChunk, priority float64) string {
	if !metadata.ReceiverDatadogEnableSamplingThresholdFeatureGate.IsEnabled() || len(chunk.GetSpans()) == 0 {
		return ""
	}
	mechanism := chunkMechanism(chunk)
	if !decidedByRate(priority, mechanism) {
		return ""
	}
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
	return samplingTraceState(logger, tags, mechanism, chunk.GetSpans()[0].TraceID)
}

// chunkTraceStateV1 returns the tracestate for the spans of a chunk in the v1 format, from the attributes of the chunk
// and of its spans. An attribute can be a number or a string, like the tags of the legacy formats.
func chunkTraceStateV1(logger *zap.Logger, chunk *idx.InternalTraceChunk) string {
	if !metadata.ReceiverDatadogEnableSamplingThresholdFeatureGate.IsEnabled() || len(chunk.Spans) == 0 || len(chunk.TraceID) != 16 {
		return ""
	}
	mechanism := chunkMechanismV1(chunk)
	if !decidedByRate(float64(chunk.Priority), mechanism) {
		return ""
	}
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
	return samplingTraceState(logger, tags, mechanism, binary.BigEndian.Uint64(chunk.TraceID[8:]))
}

// samplingTraceState returns the tracestate of the spans of a chunk that Datadog kept with the probability of the tags,
// so that components like spanmetrics can extrapolate counts from the spans. The `th` value of the `ot` member is the
// probability. Spans do not have a tracestate yet, as Datadog payloads only have one on span links.
//
// When a Knuth hash of the trace id made the decision, the `ot` member also has the `rv` value that gives the same
// decision, like the tracers derive it for the tracestate that they propagate. So `rv >= th` holds for the spans that
// were kept, and a downstream sampler does not treat them as inconsistent. Other decisions have no `rv`, because the
// trace id randomness would not agree with them.
func samplingTraceState(logger *zap.Logger, tags rateTags, mechanism samplingMechanism, traceID uint64) string {
	rate, knuth, ok := tags.probability(mechanism)
	if !ok {
		return ""
	}
	threshold, err := sampling.ProbabilityToThreshold(rate)
	if err != nil {
		// The probability is below the smallest one that a threshold can represent.
		logger.Debug("cannot convert the sampling rate to a threshold", zap.Float64("rate", rate), zap.Error(err))
		return ""
	}
	var w3c sampling.W3CTraceState
	otel := w3c.OTelValue()
	if err := otel.UpdateTValueWithSampling(threshold); err != nil {
		logger.Debug("cannot set the sampling threshold", zap.Error(err))
		return ""
	}
	if knuth {
		// Datadog keeps a trace when h = traceID * knuthFactor is at most the rate times the largest uint64, and OTel
		// when rv >= th. So rv is the inverse of h, with its low 8 bits dropped to get 56. The tag of a rate can be
		// rounded, and the 56 bits are less than the 64 of the hash, so rv is raised to th if it would be below it.
		rnd, err := sampling.UnsignedToRandomness(max((^(traceID * knuthFactor))>>8, threshold.Unsigned()))
		if err != nil {
			logger.Debug("cannot set the randomness value", zap.Error(err))
			return ""
		}
		otel.SetRValue(rnd)
	}
	var sb strings.Builder
	if err := w3c.Serialize(&sb); err != nil {
		logger.Debug("cannot serialize the tracestate", zap.Error(err))
		return ""
	}
	return sb.String()
}
