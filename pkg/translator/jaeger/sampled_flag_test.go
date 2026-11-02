// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaeger // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"

import (
	"testing"
	"time"

	model "github.com/jaegertracing/jaeger-idl/model/v1"
	"github.com/jaegertracing/jaeger-idl/thrift-gen/jaeger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func newProtoBatch(flags model.Flags) []*model.Batch {
	span := &model.Span{
		TraceID:       model.NewTraceID(0, 1),
		SpanID:        model.NewSpanID(2),
		OperationName: "op",
		StartTime:     time.Unix(0, 0),
		Duration:      time.Millisecond,
		Flags:         flags,
	}
	return []*model.Batch{{
		Process: &model.Process{ServiceName: "svc"},
		Spans:   []*model.Span{span},
	}}
}

func firstSpan(t *testing.T, td ptrace.Traces) ptrace.Span {
	t.Helper()
	require.Equal(t, 1, td.ResourceSpans().Len())
	ss := td.ResourceSpans().At(0).ScopeSpans()
	require.Equal(t, 1, ss.Len())
	require.Equal(t, 1, ss.At(0).Spans().Len())
	return ss.At(0).Spans().At(0)
}

func TestProtoSampledFlagToOTLP(t *testing.T) {
	tests := []struct {
		name        string
		flags       model.Flags
		wantSampled bool
	}{
		{"sampled", model.SampledFlag, true},
		{"not sampled", model.Flags(0), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td, err := ProtoToTraces(newProtoBatch(tt.flags))
			require.NoError(t, err)
			got := firstSpan(t, td).Flags()&spanFlagsSampled != 0
			assert.Equal(t, tt.wantSampled, got)
		})
	}
}

func TestThriftSampledFlagToOTLP(t *testing.T) {
	tests := []struct {
		name        string
		flags       int32
		wantSampled bool
	}{
		{"sampled", int32(model.SampledFlag), true},
		{"not sampled", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batch := &jaeger.Batch{
				Process: &jaeger.Process{ServiceName: "svc"},
				Spans: []*jaeger.Span{{
					TraceIdLow:    1,
					SpanId:        2,
					OperationName: "op",
					StartTime:     0,
					Duration:      1000,
					Flags:         tt.flags,
				}},
			}
			td, err := ThriftToTraces(batch)
			require.NoError(t, err)
			got := firstSpan(t, td).Flags()&spanFlagsSampled != 0
			assert.Equal(t, tt.wantSampled, got)
		})
	}
}

func TestOTLPSampledFlagToProto(t *testing.T) {
	tests := []struct {
		name        string
		otlpFlags   uint32
		wantSampled bool
	}{
		{"sampled", spanFlagsSampled, true},
		{"not sampled", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := ptrace.NewTraces()
			span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
			span.SetTraceID([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1})
			span.SetSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 2})
			span.SetName("op")
			span.SetFlags(tt.otlpFlags)

			batches := ProtoFromTraces(td)
			require.Len(t, batches, 1)
			require.Len(t, batches[0].Spans, 1)
			assert.Equal(t, tt.wantSampled, batches[0].Spans[0].Flags.IsSampled())
		})
	}
}

func TestSampledFlagRoundTrip(t *testing.T) {
	// Jaeger (sampled) -> OTLP -> Jaeger must keep the sampled bit.
	td, err := ProtoToTraces(newProtoBatch(model.SampledFlag))
	require.NoError(t, err)
	require.NotZero(t, firstSpan(t, td).Flags()&spanFlagsSampled)

	batches := ProtoFromTraces(td)
	require.Len(t, batches, 1)
	require.Len(t, batches[0].Spans, 1)
	assert.True(t, batches[0].Spans[0].Flags.IsSampled())
}
