// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package intracesampler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestNewTracesProcessor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		nextConsumer consumer.Traces
		cfg          *Config
		wantErr      bool
	}{
		{
			name: "nil_nextConsumer",
			cfg: &Config{
				SamplingPercentage: 15.5,
			},
			wantErr: true,
		},
		{
			name:         "happy_path",
			nextConsumer: consumertest.NewNop(),
			cfg: &Config{
				ScopeLeaves:        []string{"foo"},
				SamplingPercentage: 15.5,
			},
		},
		{
			name:         "happy_path_hash_seed",
			nextConsumer: consumertest.NewNop(),
			cfg: &Config{
				SamplingPercentage: 13.33,
				ScopeLeaves:        []string{"foo"},
				HashSeed:           4321,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newInTraceSamplerSpansProcessor(context.Background(), processortest.NewNopCreateSettings(), tt.cfg, tt.nextConsumer)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
			}
		})
	}
}

// when a span is in the scope list, and it is leaf, it should be unsampled (removed)
func Test_intracesampler_ScopeLeaves(t *testing.T) {
	t.Parallel()

	sink := new(consumertest.TracesSink)
	cfg := &Config{
		ScopeLeaves:        []string{"foo"},
		SamplingPercentage: 0,
	}

	// create traces with 2 spans a->b where a scope is "bar" and b scope is "foo"
	traceID := pcommon.TraceID{0, 1, 2, 3, 4, 5, 6, 7, 0, 1, 2, 3, 4, 5, 6, 7}
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-service")
	barSpan := addScopeWithOneSpan(rs, traceID, 1, "bar", nil)
	_ = addScopeWithOneSpan(rs, traceID, 2, "foo", &barSpan)

	its, err := newInTraceSamplerSpansProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, sink)
	if err != nil {
		t.Errorf("error when creating traceSamplerProcessor: %v", err)
		return
	}

	assert.NoError(t, its.ConsumeTraces(context.Background(), td))

	// check that fooSpan is removed by the processor, and we only remain with barSpan
	outputNumSpans := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().Len()
	assert.Equal(t, 1, outputNumSpans)
	outputSpan := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	assert.Equal(t, barSpan.SpanID(), outputSpan.SpanID())
}

// a span is in the scope names, but it is not removed because it is not a leaf
func Test_intracesampler_ScopeLeaves_NotLeaf(t *testing.T) {
	t.Parallel()

	sink := new(consumertest.TracesSink)
	cfg := &Config{
		ScopeLeaves:        []string{"foo"},
		SamplingPercentage: 0,
	}

	// create traces with 3 spans a->b->c
	// a scope is "bar", b scope is "foo", c scope is "baz"
	traceID := pcommon.TraceID{0, 1, 2, 3, 4, 5, 6, 7, 0, 1, 2, 3, 4, 5, 6, 7}
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-service")
	barSpan := addScopeWithOneSpan(rs, traceID, 1, "bar", nil)
	fooSpan := addScopeWithOneSpan(rs, traceID, 2, "foo", &barSpan)
	_ = addScopeWithOneSpan(rs, traceID, 3, "baz", &fooSpan)

	its, err := newInTraceSamplerSpansProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, sink)
	if err != nil {
		t.Errorf("error when creating traceSamplerProcessor: %v", err)
		return
	}

	assert.NoError(t, its.ConsumeTraces(context.Background(), td))

	// foo span should not be removed as it is not a leaf (has a child baz)
	outputNumSpans := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().Len()
	assert.Equal(t, 3, outputNumSpans)
	outputSpan := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(1).Spans().At(0)
	assert.Equal(t, fooSpan.SpanID(), outputSpan.SpanID())
}

// trace has 2 spans in a scope, and they are child-parent
// since both are leafs for this scope, they should both be removed
func Test_intracesampler_ScopeLeaves_MultipleLeafs(t *testing.T) {
	t.Parallel()

	sink := new(consumertest.TracesSink)
	cfg := &Config{
		ScopeLeaves:        []string{"foo"},
		SamplingPercentage: 0,
	}

	// create traces with 3 spans a->b->c
	// a -> scope bar, b -> scope foo, c -> scope foo
	traceID := pcommon.TraceID{0, 1, 2, 3, 4, 5, 6, 7, 0, 1, 2, 3, 4, 5, 6, 7}
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-service")
	barSpan := addScopeWithOneSpan(rs, traceID, 1, "bar", nil)
	fooParentSpan := addScopeWithOneSpan(rs, traceID, 2, "foo", &barSpan)
	_ = addScopeWithOneSpan(rs, traceID, 3, "foo", &fooParentSpan)

	its, err := newInTraceSamplerSpansProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, sink)
	if err != nil {
		t.Errorf("error when creating traceSamplerProcessor: %v", err)
		return
	}

	assert.NoError(t, its.ConsumeTraces(context.Background(), td))

	// both foo spans should have been removed
	outputNumSpans := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().Len()
	assert.Equal(t, 1, outputNumSpans)
	outputSpan := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	assert.Equal(t, barSpan.SpanID(), outputSpan.SpanID())
}

// when the processor is not preceded with a groupbytrace processor
// or tailprocessor, than the tree it builds is not complete and sampling can
// cause issues when other spans arrive
// this test asserts that when the processor got spans which are not all from same trace
// it does not attempt to process these spans
func Test_intracesampler_MultipleTraces(t *testing.T) {
	t.Parallel()

	sink := new(consumertest.TracesSink)
	cfg := &Config{
		ScopeLeaves:        []string{"foo"},
		SamplingPercentage: 0,
	}

	// create 2 traces: 1 with 2 spans a->b, other with span a
	// a -> scope bar, b -> scope foo
	// if this was the only trace it would be sampled, removing span b
	traceID1 := pcommon.TraceID{0, 1, 2, 3, 4, 5, 6, 7, 0, 1, 2, 3, 4, 5, 6, 7}
	traceID2 := pcommon.TraceID{1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4}
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-service")
	barSpan1 := addScopeWithOneSpan(rs, traceID1, 1, "bar", nil)
	fooSpan1 := addScopeWithOneSpan(rs, traceID1, 2, "foo", &barSpan1)
	_ = addScopeWithOneSpan(rs, traceID2, 1, "bar", nil)

	its, err := newInTraceSamplerSpansProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, sink)
	if err != nil {
		t.Errorf("error when creating traceSamplerProcessor: %v", err)
		return
	}

	assert.NoError(t, its.ConsumeTraces(context.Background(), td))

	// both foo spans should have been removed
	outputNumScopes := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().Len()
	assert.Equal(t, 3, outputNumScopes)
	outputFooSpan := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(1).Spans().At(0)
	assert.Equal(t, fooSpan1.SpanID(), outputFooSpan.SpanID())
}

func Test_intracesampler_Empty(t *testing.T) {
	t.Parallel()

	sink := new(consumertest.TracesSink)
	cfg := &Config{
		ScopeLeaves:        []string{"foo"},
		SamplingPercentage: 0,
	}

	td := ptrace.NewTraces()

	its, err := newInTraceSamplerSpansProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, sink)
	if err != nil {
		t.Errorf("error when creating traceSamplerProcessor: %v", err)
		return
	}

	assert.NoError(t, its.ConsumeTraces(context.Background(), td))
}

func Test_intracesampler_ScopeLeaves_MultipleScopes(t *testing.T) {
	t.Parallel()

	traceID := pcommon.TraceID{0, 1, 2, 3, 4, 5, 6, 7, 0, 1, 2, 3, 4, 5, 6, 7}

	sink := new(consumertest.TracesSink)
	cfg := &Config{
		ScopeLeaves:        []string{"foo", "baz"},
		SamplingPercentage: 0,
	}

	// create traces with 3 spans a->b->c
	// a scope is "bar", b scope is "foo", c scope is "baz"
	// foo and baz should be unsampled
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-service")
	barSpan := addScopeWithOneSpan(rs, traceID, 1, "bar", nil)
	fooSpan := addScopeWithOneSpan(rs, traceID, 2, "foo", &barSpan)
	_ = addScopeWithOneSpan(rs, traceID, 3, "baz", &fooSpan)

	its, err := newInTraceSamplerSpansProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, sink)
	if err != nil {
		t.Errorf("error when creating traceSamplerProcessor: %v", err)
		return
	}

	assert.NoError(t, its.ConsumeTraces(context.Background(), td))

	// foo and baz spans should be removed since baz is leaf and foo is leaf after baz is unsampled
	outputNumSpans := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().Len()
	assert.Equal(t, 1, outputNumSpans)
	outputSpan := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	assert.Equal(t, barSpan.SpanID(), outputSpan.SpanID())
}

func Test_intracesampler_SamplingPercentage(t *testing.T) {
	t.Parallel()

	traceID := pcommon.TraceID{0, 1, 2, 3, 4, 5, 6, 7, 0, 1, 2, 3, 4, 5, 6, 7}

	sink := new(consumertest.TracesSink)
	cfg := &Config{
		ScopeLeaves:        []string{"foo"},
		SamplingPercentage: 100,
	}

	// create traces with 2 spans a->b where a scope is "bar" and b scope is "foo"
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-service")
	barSpan := addScopeWithOneSpan(rs, traceID, 1, "bar", nil)
	_ = addScopeWithOneSpan(rs, traceID, 2, "foo", &barSpan)

	its, err := newInTraceSamplerSpansProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, sink)
	if err != nil {
		t.Errorf("error when creating traceSamplerProcessor: %v", err)
		return
	}

	assert.NoError(t, its.ConsumeTraces(context.Background(), td))

	// foo span should not be removed since sampling percentage is 100%
	// thus all spans should be kept
	outputNumSpans := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().Len()
	assert.Equal(t, 2, outputNumSpans)
}

func addScopeWithOneSpan(rs ptrace.ResourceSpans, traceID pcommon.TraceID, spanIndex byte, scopeName string, parentSpan *ptrace.Span) ptrace.Span {
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName(scopeName)
	span := ss.Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID(pcommon.SpanID{spanIndex, spanIndex, spanIndex, spanIndex, spanIndex, spanIndex, spanIndex, spanIndex})
	if parentSpan != nil {
		span.SetParentSpanID(parentSpan.SpanID())
	}
	return span
}
