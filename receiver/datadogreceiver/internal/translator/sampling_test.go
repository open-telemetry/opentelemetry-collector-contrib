// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator"

import (
	"net/http"
	"testing"

	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	ddsampler "github.com/DataDog/datadog-agent/pkg/trace/sampler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/metadata"
)

// enableSamplingThreshold turns the alpha feature gate of the sampling threshold on for a test.
func enableSamplingThreshold(t *testing.T) {
	t.Helper()
	id := metadata.ReceiverDatadogEnableSamplingThresholdFeatureGate.ID()
	require.NoError(t, featuregate.GlobalRegistry().Set(id, true))
	t.Cleanup(func() { require.NoError(t, featuregate.GlobalRegistry().Set(id, false)) })
}

// The Datadog trace id 2 has this randomness value, which is the inverse of the Knuth hash of the id, 56 bits of it. It
// is derived independently of the code: 2 * 1111111111111111111 mod 2^64 = 2222222222222222222, its inverse is
// 2^64-1-2222222222222222222, and the top 56 bits of that are 0xe12914a9a8771c. A trace id of 2 is kept by the Knuth
// hash at any rate above 0.12, because its hash is 0.12 of the range. The ids 8 and 9 are on both sides of a rate of
// 0.5: id 8 has the hash 0.48 and is kept, id 9 has 0.54 and is not.
const (
	rvID2 = "e12914a9a8771c"
	rvID8 = "84a452a6a1dc71"
)

func tracestatesBySpanID(t *testing.T, payload *pb.TracerPayload) map[uint64]string {
	t.Helper()
	traces, err := ToTraces(zap.NewNop(), payload, &http.Request{Header: http.Header{}}, nil)
	require.NoError(t, err)
	got := map[uint64]string{}
	for _, rs := range traces.ResourceSpans().All() {
		for _, ss := range rs.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				got[spanIDToUint64(span.SpanID())] = span.TraceState().AsRaw()
			}
		}
	}
	return got
}

func spanIDToUint64(id [8]byte) (v uint64) {
	for _, b := range id {
		v = v<<8 | uint64(b)
	}
	return v
}

func TestToTracesSamplingThreshold(t *testing.T) {
	enableSamplingThreshold(t)
	keep := int32(ddsampler.PriorityAutoKeep)
	// A chunk of a deciding root and a child, the way a tracer sends it.
	chunk := func(priority int32, root, child *pb.Span) *pb.TraceChunk {
		root.SpanID, root.TraceID = 1, 2
		child.SpanID, child.TraceID, child.ParentID = 2, 2, 1
		return &pb.TraceChunk{Priority: priority, Spans: []*pb.Span{child, root}}
	}

	tests := []struct {
		name   string
		chunks []*pb.TraceChunk
		want   map[uint64]string
	}{
		{
			name: "the agent rate is preferred over the Knuth rate on another span",
			chunks: []*pb.TraceChunk{chunk(keep,
				&pb.Span{Metrics: map[string]float64{"_dd.agent_psr": 0.5}},
				&pb.Span{Meta: map[string]string{"_dd.p.ksr": "0.25"}})},
			want: map[uint64]string{1: "ot=rv:" + rvID2 + ";th:8", 2: "ot=rv:" + rvID2 + ";th:8"},
		},
		{
			name: "the Knuth rate on one span applies to the whole chunk",
			chunks: []*pb.TraceChunk{chunk(keep,
				&pb.Span{},
				&pb.Span{Meta: map[string]string{"_dd.p.ksr": "0.25"}})},
			want: map[uint64]string{1: "ot=rv:" + rvID2 + ";th:c", 2: "ot=rv:" + rvID2 + ";th:c"},
		},
		{
			name: "the Knuth rate is preferred over the rule rate",
			chunks: []*pb.TraceChunk{chunk(keep,
				&pb.Span{Metrics: map[string]float64{"_dd.rule_psr": 0.5}, Meta: map[string]string{"_dd.p.ksr": "0.25"}},
				&pb.Span{})},
			want: map[uint64]string{1: "ot=rv:" + rvID2 + ";th:c", 2: "ot=rv:" + rvID2 + ";th:c"},
		},
		{
			name: "the rule rate is multiplied by the limit rate",
			chunks: []*pb.TraceChunk{chunk(keep,
				&pb.Span{Metrics: map[string]float64{"_dd.rule_psr": 0.5, "_dd.limit_psr": 0.5}},
				&pb.Span{})},
			want: map[uint64]string{1: "ot=th:c", 2: "ot=th:c"},
		},
		{
			name: "the rule rate alone is used without a limit rate",
			chunks: []*pb.TraceChunk{chunk(keep,
				&pb.Span{Metrics: map[string]float64{"_dd.rule_psr": 0.5}},
				&pb.Span{})},
			want: map[uint64]string{1: "ot=th:8", 2: "ot=th:8"},
		},
		{
			name: "the Knuth rate is multiplied by the limiter rate, which can be on another span",
			chunks: []*pb.TraceChunk{chunk(int32(ddsampler.PriorityUserKeep),
				&pb.Span{Metrics: map[string]float64{"_dd.rule_psr": 0.5, "_dd.limit_psr": 0.5}, Meta: map[string]string{"_dd.p.dm": "-3"}},
				&pb.Span{Meta: map[string]string{"_dd.p.ksr": "0.5"}})},
			want: map[uint64]string{1: "ot=th:c", 2: "ot=th:c"},
		},
		{
			name: "the Knuth rate is not multiplied by the limit of a tracer that sets the configured limit",
			chunks: []*pb.TraceChunk{chunk(int32(ddsampler.PriorityUserKeep),
				&pb.Span{Metrics: map[string]float64{"_dd.rule_psr": 0.5, "_dd.limit_psr": 100}, Meta: map[string]string{"_dd.p.dm": "-3"}},
				&pb.Span{Meta: map[string]string{"_dd.p.ksr": "0.5"}})},
			want: map[uint64]string{1: "ot=rv:" + rvID2 + ";th:8", 2: "ot=rv:" + rvID2 + ";th:8"},
		},
		{
			name: "the agent rate is not multiplied by a limiter rate",
			chunks: []*pb.TraceChunk{chunk(keep,
				&pb.Span{Metrics: map[string]float64{"_dd.agent_psr": 0.5, "_dd.limit_psr": 0.5}}, &pb.Span{})},
			want: map[uint64]string{1: "ot=rv:" + rvID2 + ";th:8", 2: "ot=rv:" + rvID2 + ";th:8"},
		},
		{
			name: "a rate of 1 keeps everything",
			chunks: []*pb.TraceChunk{chunk(keep,
				&pb.Span{Metrics: map[string]float64{"_dd.agent_psr": 1}},
				&pb.Span{})},
			want: map[uint64]string{1: "ot=rv:" + rvID2 + ";th:0", 2: "ot=rv:" + rvID2 + ";th:0"},
		},
		{
			name: "the priority comes from the spans of the older formats, without a priority on the chunk",
			chunks: []*pb.TraceChunk{chunk(int32(ddsampler.PriorityNone),
				&pb.Span{Metrics: map[string]float64{"_sampling_priority_v1": 1, "_dd.agent_psr": 0.5}},
				&pb.Span{})},
			want: map[uint64]string{1: "ot=rv:" + rvID2 + ";th:8", 2: "ot=rv:" + rvID2 + ";th:8"},
		},
		{
			name: "the rate can be a tag of the chunk",
			chunks: []*pb.TraceChunk{func() *pb.TraceChunk {
				c := chunk(keep, &pb.Span{}, &pb.Span{})
				c.Tags = map[string]string{"_dd.p.ksr": "0.25"}
				return c
			}()},
			want: map[uint64]string{1: "ot=rv:" + rvID2 + ";th:c", 2: "ot=rv:" + rvID2 + ";th:c"},
		},
		{
			name: "a user keep is not decided by a rate",
			chunks: []*pb.TraceChunk{chunk(int32(ddsampler.PriorityUserKeep),
				&pb.Span{Metrics: map[string]float64{"_dd.agent_psr": 0.5}}, &pb.Span{})},
			want: map[uint64]string{1: "", 2: ""},
		},
		{
			name: "a user keep by a sampling rule of the tracer is decided by its rate",
			chunks: []*pb.TraceChunk{chunk(int32(ddsampler.PriorityUserKeep),
				&pb.Span{Metrics: map[string]float64{"_dd.rule_psr": 0.5}, Meta: map[string]string{"_dd.p.dm": "-3"}}, &pb.Span{})},
			want: map[uint64]string{1: "ot=th:8", 2: "ot=th:8"},
		},
		{
			name: "the limit of a tracer that sets the configured limit instead of a rate is ignored",
			chunks: []*pb.TraceChunk{chunk(int32(ddsampler.PriorityUserKeep),
				&pb.Span{Metrics: map[string]float64{"_dd.rule_psr": 0.5, "_dd.limit_psr": 100}, Meta: map[string]string{"_dd.p.dm": "-3"}}, &pb.Span{})},
			want: map[uint64]string{1: "ot=th:8", 2: "ot=th:8"},
		},
		{
			name: "the decision maker can be a tag of the chunk",
			chunks: []*pb.TraceChunk{func() *pb.TraceChunk {
				c := chunk(int32(ddsampler.PriorityUserKeep), &pb.Span{Metrics: map[string]float64{"_dd.rule_psr": 0.5}}, &pb.Span{})
				c.Tags = map[string]string{"_dd.p.dm": "-11"}
				return c
			}()},
			want: map[uint64]string{1: "ot=th:8", 2: "ot=th:8"},
		},
		{
			name: "a manual keep is not decided by a rate, even when it has one",
			chunks: []*pb.TraceChunk{chunk(int32(ddsampler.PriorityUserKeep),
				&pb.Span{Metrics: map[string]float64{"_dd.agent_psr": 0.5}, Meta: map[string]string{"_dd.p.dm": "-4"}}, &pb.Span{})},
			want: map[uint64]string{1: "", 2: ""},
		},
		{
			name: "a drop is not decided by a rate, whatever the decision maker",
			chunks: []*pb.TraceChunk{chunk(int32(ddsampler.PriorityUserDrop),
				&pb.Span{Metrics: map[string]float64{"_dd.rule_psr": 0.5}, Meta: map[string]string{"_dd.p.dm": "-3"}}, &pb.Span{})},
			want: map[uint64]string{1: "", 2: ""},
		},
		{
			name: "the rate of the probabilistic sampler of the Agent",
			chunks: []*pb.TraceChunk{chunk(keep,
				&pb.Span{Metrics: map[string]float64{"_dd.prob_sr": 0.25}, Meta: map[string]string{"_dd.p.dm": "-9"}}, &pb.Span{})},
			want: map[uint64]string{1: "ot=th:c", 2: "ot=th:c"},
		},
		{
			name: "the probabilistic sampler of the Agent overrides the agent rate of the tracer, and has no randomness value",
			chunks: []*pb.TraceChunk{chunk(keep,
				&pb.Span{Metrics: map[string]float64{"_dd.agent_psr": 0.5, "_dd.prob_sr": 0.25}, Meta: map[string]string{"_dd.p.dm": "-9"}}, &pb.Span{})},
			want: map[uint64]string{1: "ot=th:c", 2: "ot=th:c"},
		},
		{
			name: "the probabilistic sampler of the Agent without its rate leaves the rate of the tracer unused",
			chunks: []*pb.TraceChunk{chunk(keep,
				&pb.Span{Metrics: map[string]float64{"_dd.agent_psr": 0.5}, Meta: map[string]string{"_dd.p.dm": "-9"}}, &pb.Span{})},
			want: map[uint64]string{1: "", 2: ""},
		},
		{
			// Trace id 8 is kept by the Knuth hash at a rate of 0.5, and its randomness is just above the threshold.
			name: "the randomness value follows the Knuth hash of the lower 64 bits of the trace id",
			chunks: []*pb.TraceChunk{{Priority: keep, Spans: []*pb.Span{{
				TraceID: 8, SpanID: 1, Metrics: map[string]float64{"_dd.agent_psr": 0.5},
			}}}},
			want: map[uint64]string{1: "ot=rv:" + rvID8 + ";th:8"},
		},
		{
			// Trace id 9 is not kept by the Knuth hash at a rate of 0.5, so its randomness would be below the threshold.
			// That cannot happen for an exact rate, but it can just at the edge for a rounded one (the Knuth rate tag has 6
			// decimals), and this id stands in for that. The randomness is raised to the threshold, so that a span that
			// was kept is never inconsistent.
			name: "the randomness value of a kept chunk is not below the threshold",
			chunks: []*pb.TraceChunk{{Priority: keep, Spans: []*pb.Span{{
				TraceID: 9, SpanID: 1, Metrics: map[string]float64{"_dd.agent_psr": 0.5},
			}}}},
			want: map[uint64]string{1: "ot=rv:80000000000000;th:8"},
		},
		{
			name: "a dropped chunk, kept for its error, is not decided by a rate",
			chunks: []*pb.TraceChunk{chunk(int32(ddsampler.PriorityAutoDrop),
				&pb.Span{Metrics: map[string]float64{"_dd.agent_psr": 0.5}}, &pb.Span{})},
			want: map[uint64]string{1: "", 2: ""},
		},
		{
			name:   "no priority",
			chunks: []*pb.TraceChunk{chunk(int32(ddsampler.PriorityNone), &pb.Span{Metrics: map[string]float64{"_dd.agent_psr": 0.5}}, &pb.Span{})},
			want:   map[uint64]string{1: "", 2: ""},
		},
		{
			name:   "no rate",
			chunks: []*pb.TraceChunk{chunk(keep, &pb.Span{}, &pb.Span{})},
			want:   map[uint64]string{1: "", 2: ""},
		},
		{
			name: "rates that are not a probability are ignored",
			chunks: []*pb.TraceChunk{chunk(keep,
				&pb.Span{Metrics: map[string]float64{"_dd.agent_psr": 1.5, "_dd.rule_psr": 0}},
				&pb.Span{Meta: map[string]string{"_dd.p.ksr": "not a number"}})},
			want: map[uint64]string{1: "", 2: ""},
		},
		{
			// The first chunk of a partial flush is the one with the deciding root. Each chunk repeats the Knuth rate.
			name: "every chunk of a partially flushed trace gets a threshold",
			chunks: []*pb.TraceChunk{
				{Priority: keep, Spans: []*pb.Span{{TraceID: 2, SpanID: 1, Metrics: map[string]float64{"_dd.agent_psr": 0.5}}}},
				{Priority: keep, Spans: []*pb.Span{{TraceID: 2, SpanID: 2, ParentID: 1, Meta: map[string]string{"_dd.p.ksr": "0.5"}}}},
			},
			want: map[uint64]string{1: "ot=rv:" + rvID2 + ";th:8", 2: "ot=rv:" + rvID2 + ";th:8"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tracestatesBySpanID(t, &pb.TracerPayload{Chunks: tt.chunks}))
		})
	}

	t.Run("the threshold decodes to the Datadog rate", func(t *testing.T) {
		// The Knuth rate of a real Java tracer has 6 decimals, like th:fdb0a068f801d8 in the same trace.
		const rate = 0.009024
		payload := &pb.TracerPayload{Chunks: []*pb.TraceChunk{chunk(keep, &pb.Span{Meta: map[string]string{"_dd.p.ksr": "0.009024"}}, &pb.Span{})}}
		w3c, err := sampling.NewW3CTraceState(tracestatesBySpanID(t, payload)[1])
		require.NoError(t, err)
		threshold, ok := w3c.OTelValue().TValueThreshold()
		require.True(t, ok)
		assert.InDelta(t, rate, threshold.Probability(), rate*1e-9)
	})
}

func TestToTracesSamplingThresholdFeatureGate(t *testing.T) {
	payload := &pb.TracerPayload{Chunks: []*pb.TraceChunk{{
		Priority: int32(ddsampler.PriorityAutoKeep),
		Spans:    []*pb.Span{{TraceID: 2, SpanID: 1, Metrics: map[string]float64{"_dd.agent_psr": 0.5}}},
	}}}
	assert.Equal(t, map[uint64]string{1: ""}, tracestatesBySpanID(t, payload), "the feature gate is off by default")

	enableSamplingThreshold(t)
	assert.Equal(t, map[uint64]string{1: "ot=rv:" + rvID2 + ";th:8"}, tracestatesBySpanID(t, payload))
}
