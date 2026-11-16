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
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
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
	keep := int32(ddsampler.PriorityAutoKeep)
	// A chunk of a deciding root and a child, the way a tracer sends it.
	chunk := func(priority int32, root, child *pb.Span) *pb.TraceChunk {
		root.SpanID, root.TraceID = 1, 7
		child.SpanID, child.TraceID, child.ParentID = 2, 7, 1
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
			want: map[uint64]string{1: "ot=th:8", 2: "ot=th:8"},
		},
		{
			name: "the Knuth rate on one span applies to the whole chunk",
			chunks: []*pb.TraceChunk{chunk(keep,
				&pb.Span{},
				&pb.Span{Meta: map[string]string{"_dd.p.ksr": "0.25"}})},
			want: map[uint64]string{1: "ot=th:c", 2: "ot=th:c"},
		},
		{
			name: "the Knuth rate is preferred over the rule rate",
			chunks: []*pb.TraceChunk{chunk(keep,
				&pb.Span{Metrics: map[string]float64{"_dd.rule_psr": 0.5}, Meta: map[string]string{"_dd.p.ksr": "0.25"}},
				&pb.Span{})},
			want: map[uint64]string{1: "ot=th:c", 2: "ot=th:c"},
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
			want: map[uint64]string{1: "ot=th:8", 2: "ot=th:8"},
		},
		{
			name: "the agent rate is not multiplied by a limiter rate",
			chunks: []*pb.TraceChunk{chunk(keep,
				&pb.Span{Metrics: map[string]float64{"_dd.agent_psr": 0.5, "_dd.limit_psr": 0.5}}, &pb.Span{})},
			want: map[uint64]string{1: "ot=th:8", 2: "ot=th:8"},
		},
		{
			name: "a rate of 1 keeps everything",
			chunks: []*pb.TraceChunk{chunk(keep,
				&pb.Span{Metrics: map[string]float64{"_dd.agent_psr": 1}},
				&pb.Span{})},
			want: map[uint64]string{1: "ot=th:0", 2: "ot=th:0"},
		},
		{
			name: "the priority comes from the spans of the older formats, without a priority on the chunk",
			chunks: []*pb.TraceChunk{chunk(int32(ddsampler.PriorityNone),
				&pb.Span{Metrics: map[string]float64{"_sampling_priority_v1": 1, "_dd.agent_psr": 0.5}},
				&pb.Span{})},
			want: map[uint64]string{1: "ot=th:8", 2: "ot=th:8"},
		},
		{
			name: "the rate can be a tag of the chunk",
			chunks: []*pb.TraceChunk{func() *pb.TraceChunk {
				c := chunk(keep, &pb.Span{}, &pb.Span{})
				c.Tags = map[string]string{"_dd.p.ksr": "0.25"}
				return c
			}()},
			want: map[uint64]string{1: "ot=th:c", 2: "ot=th:c"},
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
				{Priority: keep, Spans: []*pb.Span{{TraceID: 7, SpanID: 1, Metrics: map[string]float64{"_dd.agent_psr": 0.5}}}},
				{Priority: keep, Spans: []*pb.Span{{TraceID: 7, SpanID: 2, ParentID: 1, Meta: map[string]string{"_dd.p.ksr": "0.5"}}}},
			},
			want: map[uint64]string{1: "ot=th:8", 2: "ot=th:8"},
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

// TestSetSamplingThresholdFollowsTracestateSpec covers a tracestate that already has a threshold, which is the combined
// probability of the earlier sampling stages. The rate of Datadog is 0.5 in all cases.
// See https://opentelemetry.io/docs/specs/otel/trace/tracestate-probability-sampling/.
func TestSetSamplingThresholdFollowsTracestateSpec(t *testing.T) {
	tests := []struct {
		name, tracestate, want string
	}{
		{"empty", "", "ot=th:8"},
		{"other vendors are kept", "vendor=x,other=y", "ot=th:8,vendor=x,other=y"},
		{"randomness and other keys of ot are kept", "ot=rv:abcdefabcdefab;x:y,vendor=x", "ot=rv:abcdefabcdefab;th:8;x:y,vendor=x"},
		// 0.5 * 0.5 = 0.25, which is threshold c, and 0.25 * 0.5 = 0.125, which is threshold e.
		{"a threshold is multiplied with the rate", "ot=th:8,vendor=x", "ot=th:c,vendor=x"},
		{"a threshold with randomness is multiplied with the rate", "ot=rv:fedcba98765432;th:c", "ot=rv:fedcba98765432;th:e"},
		// Randomness is kept when it is equal to the threshold, because a span is kept when randomness >= threshold.
		{"randomness equal to the threshold is consistent", "ot=rv:80000000000000;th:8", "ot=rv:80000000000000;th:c"},
		{"randomness below the threshold is inconsistent, so the threshold is erased", "ot=rv:7fffffffffffff;th:8,vendor=x", "ot=rv:7fffffffffffff,vendor=x"},
		{"a threshold that becomes too small is erased", "ot=th:ffffffffffffff,vendor=x", "vendor=x"},
		{"an invalid tracestate is not overwritten", "ot==,", "ot==,"},
		{"an invalid threshold is not overwritten", "ot=th:zz", "ot=th:zz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			span := ptrace.NewSpan()
			span.TraceState().FromRaw(tt.tracestate)
			setSamplingThreshold(zap.NewNop(), span, 0.5)
			assert.Equal(t, tt.want, span.TraceState().AsRaw())
		})
	}

	t.Run("a rate of 1 keeps the exact earlier threshold", func(t *testing.T) {
		span := ptrace.NewSpan()
		span.TraceState().FromRaw("ot=th:fdb0a068f801d8")
		setSamplingThreshold(zap.NewNop(), span, 1)
		assert.Equal(t, "ot=th:fdb0a068f801d8", span.TraceState().AsRaw())
	})
}
