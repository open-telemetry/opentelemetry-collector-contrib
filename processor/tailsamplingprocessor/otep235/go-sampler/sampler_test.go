// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package sampler

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var testAttrs = []attribute.KeyValue{attribute.String("K", "V")}

var testTs = func() trace.TraceState {
	ts, err := trace.ParseTraceState("tsk1=tsv,tsk2=tsv")
	if err != nil {
		panic(err)
	}
	return ts
}()

func TestSamplerDescription(t *testing.T) {
	type testCase struct {
		sampler     interface{ Description() string }
		description string
	}

	for _, test := range []testCase{
		{
			AlwaysSample(),
			"AlwaysOn",
		},
		{
			NeverSample(),
			"AlwaysOff",
		},
		{
			RuleBased(
				WithRule(SpanNamePredicate("/healthcheck"), ComposableNeverSample()),
				WithDefaultRule(ComposableAlwaysSample()),
			),
			"RuleBased{rule(Span.Name==/healthcheck)=AlwaysOff,rule(true)=AlwaysOn}",
		},
		{
			ComposableParentBased(ComposableAlwaysSample()),
			"RuleBased{rule(root?)=AlwaysOn,rule(true)=ParentThreshold}",
		},
	} {
		require.Equal(t, test.description, test.sampler.Description())
	}
}

type samplerAnd[T any] struct {
	sampler Sampler
	data    T
}

func (sand samplerAnd[T]) name() string {
	return fmt.Sprintf("%T:%v:%v", sand.sampler, sand.sampler.Description(), sand.data)
}

type samplerAnd2[T1 any, T2 any] struct {
	sampler Sampler
	data1   T1
	data2   T2
}

func (sand samplerAnd2[T1, T2]) name() string {
	return fmt.Sprintf("%T:%v:%v:%v", sand.sampler, sand.sampler.Description(), sand.data1, sand.data2)
}

// TestChildSpanUnknownThresholdSampled tests a sampled child span
// sampled context w/o tracestate.  All will sample this span.  The
// boolean indicates whether the threshold is output.  Only the
// consistent always-on case will restore a known threshold.
func TestChildSpanUnknownThresholdSampled(t *testing.T) {
	for _, sand := range []samplerAnd[bool]{
		{AlwaysSample(), false},
		{CompositeSampler(ComposableAlwaysSample()), true},
		{ParentBased(AlwaysSample()), false},
		{CompositeSampler(ComposableParentBased(ComposableAlwaysSample())), false},
	} {
		t.Run(sand.name(), func(t *testing.T) {
			test := defaultTestFuncs()
			test.tracestate = func() trace.TraceState {
				return testTs
			}
			ts := testTs

			// Consistent AlwaysOn samplers introduce th:0.
			if sand.data {
				ts = testTsWith("th:0")
			}

			// tracestater is always unmodified because the incoming
			// threshold is unknown
			params := makeTestContext(test).SamplingParameters

			result := sand.sampler.ShouldSample(params)
			require.Equal(t, RecordAndSample, result.Decision)
			require.Empty(t, result.Attributes)
			require.Equal(t, ts, result.Tracestate)
		})
	}
}

// TestChildSpanUnknownThresholdNotSampled tests a sampled child span
// unsampled context w/o tracestate.  All will not sample this span.
func TestChildSpanUnknownThresholdNotSampled(t *testing.T) {
	for _, sand := range []samplerAnd[bool]{
		{ParentBased(AlwaysSample()), false},
		{CompositeSampler(ComposableParentBased(ComposableAlwaysSample())), false},
	} {
		t.Run(sand.name(), func(t *testing.T) {
			test := defaultTestFuncs()
			test.sampled = func() bool {
				return false
			}
			test.tracestate = func() trace.TraceState {
				return testTs
			}
			ts := testTs

			// tracestater is always unmodified because the incoming
			// threshold is unknown
			params := makeTestContext(test).SamplingParameters

			result := sand.sampler.ShouldSample(params)
			require.Equal(t, Drop, result.Decision)
			require.Empty(t, result.Attributes)
			require.Equal(t, ts, result.Tracestate)
		})
	}
}

// TestChildSpanInvalidThresholdSampled tests samplers
// in a valid-threshold, sampled child context.
func TestChildSpanValidThresholdSampled(t *testing.T) {
	for _, sampler := range []Sampler{
		AlwaysSample(),
		CompositeSampler(ComposableAlwaysSample()),
		ParentBased(AlwaysSample()),
		ParentBased(AlwaysSample()),
		CompositeSampler(ComposableParentBased(ComposableAlwaysSample())),
	} {
		// These tests run in a child span context with valid threshold.
		// All sample, all produce threshold.
		t.Run(fmt.Sprintf("%T:%v", sampler, sampler.Description()), func(t *testing.T) {
			ts100 := testTsWith("th:0")
			test := defaultTestFuncs()
			test.tracestate = func() trace.TraceState {
				return ts100
			}

			// tracestate is always unmodified because the incoming
			// threshold was known.
			params := makeTestContext(test).SamplingParameters

			result := sampler.ShouldSample(params)
			require.Equal(t, RecordAndSample, result.Decision)
			require.Empty(t, result.Attributes)
			require.Equal(t, ts100, result.Tracestate)
		})
	}
}

// TestChildSpanInvalidThresholdNotSampled tests the case where the sampled flag
// and threshold disagree and the sampled flag is FALSE.
func TestChildSpanInvalidThresholdNotSampled(t *testing.T) {
	for _, sand := range []samplerAnd[bool]{
		{ParentBased(AlwaysSample()), false},
		{CompositeSampler(ComposableParentBased(ComposableAlwaysSample())), true},
	} {
		// These tests run in a child span context.
		t.Run(sand.name(), func(t *testing.T) {
			test := defaultTestFuncs()
			test.sampled = func() bool {
				// saying not sampled
				return false
			}
			// saying 100% sampled
			ts100 := testTsWith("th:0")

			test.tracestate = func() trace.TraceState {
				return ts100
			}
			params := makeTestContext(test).SamplingParameters

			result := sand.sampler.ShouldSample(params)
			if sand.data {
				require.Equal(t, RecordAndSample, result.Decision)
			} else {
				require.Equal(t, Drop, result.Decision)
			}
			require.Empty(t, result.Attributes)
			require.Equal(t, ts100, result.Tracestate)
		})
	}
}

// TestSampledInvalidThreshold tests the case where the sampled flag
// and threshold disagree and the sampled flag is TRUE.
func TestChildSpanInvalidThresholdSampled(t *testing.T) {
	for _, sand := range []samplerAnd[bool]{
		{ParentBased(AlwaysSample()), false},
		{CompositeSampler(ComposableParentBased(ComposableAlwaysSample())), true},
	} {
		// These tests run in a child span context.
		t.Run(sand.name(), func(t *testing.T) {
			test := defaultTestFuncs()
			test.sampled = func() bool {
				// saying sampled
				return true
			}
			// saying 2^-56 sampled; assume no traceID matches.
			tsFF := testTsWith("th:ffffffffffffff")

			test.tracestate = func() trace.TraceState {
				return tsFF
			}
			params := makeTestContext(test).SamplingParameters

			result := sand.sampler.ShouldSample(params)
			// Both sample
			require.Equal(t, RecordAndSample, result.Decision)
			if sand.data {
				// the invalid threshold is erased,
				require.Equal(t, testTs, result.Tracestate)
			} else {
				// the invalid threshold is NOT erased
				require.Equal(t, tsFF, result.Tracestate)
			}
			require.Empty(t, result.Attributes)
		})
	}
}

// TestAnnotatingSampler tests that sampler-conditioned attributes work.
func TestAnnotatingSampler(t *testing.T) {
	var tatts = []attribute.KeyValue{
		attribute.String("extra1", "1"),
		attribute.Int("extra2", 2),
	}
	for _, sampler := range []Sampler{
		CompositeSampler(
			AnnotatingSampler(
				ComposableParentBased(ComposableAlwaysSample()),
				WithSampledAttributes(func() []attribute.KeyValue {
					return tatts
				})),
		),
		CompositeSampler(
			AnnotatingSampler(
				ComposableAlwaysSample(),
				WithSampledAttributes(func() []attribute.KeyValue {
					return tatts
				})),
		),
	} {
		t.Run(fmt.Sprintf("%T:%v", sampler, sampler.Description()), func(t *testing.T) {
			ts100 := testTsWith("th:0")
			test := defaultTestFuncs()
			test.tracestate = func() trace.TraceState {
				return ts100
			}
			params := makeTestContext(test).SamplingParameters

			result := sampler.ShouldSample(params)
			require.Equal(t, RecordAndSample, result.Decision)
			require.Equal(t, tatts, result.Attributes)
			require.Equal(t, ts100, result.Tracestate)
		})
	}
}

func TestTraceIdRatioBased(t *testing.T) {
	yes := trace.TraceID{0, 0, 0, 0, 0, 0, 0, 0, 0, 0x80, 0, 0, 0, 0, 0, 0}
	no := trace.TraceID{0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	for _, sand := range []samplerAnd2[trace.TraceID, bool]{
		{CompositeSampler(TraceIDRatioBased(0.5)), no, false},
		{CompositeSampler(TraceIDRatioBased(0.5)), yes, true},
		{CompositeSampler(ComposableParentBased(TraceIDRatioBased(0.5))), no, false},
		{CompositeSampler(ComposableParentBased(TraceIDRatioBased(0.5))), yes, true},
	} {
		t.Run(sand.name(), func(t *testing.T) {

			test := defaultTestFuncs()
			test.tracestate = func() trace.TraceState {
				return testTs
			}
			test.sampled = func() bool { return false }
			test.parentid = func(*rand.Rand) trace.TraceID {
				// These tests run in a root span context.
				return trace.TraceID{}
			}
			test.traceid = func(*rand.Rand) trace.TraceID {
				// These tests run in a root span context.
				return sand.data1
			}

			params := makeTestContext(test).SamplingParameters

			result := sand.sampler.ShouldSample(params)
			// Both sample
			if sand.data2 {
				require.Equal(t, RecordAndSample, result.Decision)

				// tracestate has ot=th:8
				require.Equal(t, testTsWith("th:8"), result.Tracestate)
			} else {
				require.Equal(t, Drop, result.Decision)

				// tracestate unmodified b/c not sampled
				require.Equal(t, testTs, result.Tracestate)
			}
			require.Empty(t, result.Attributes)
		})
	}
}

func testTsWith(otts string) trace.TraceState {
	mod, err := testTs.Insert("ot", otts)
	if err != nil {
		panic(fmt.Errorf("impossible test fail: %w", err))
	}
	return mod
}

type testContext struct {
	context.Context
	SamplingParameters
}

type testFuncs struct {
	parentid     func(*rand.Rand) trace.TraceID
	traceid      func(*rand.Rand) trace.TraceID
	sampled      func() bool
	remote       func() bool
	tracestate   func() trace.TraceState
	name         func() string
	kind         func() trace.SpanKind
	inAttributes []attribute.KeyValue
	links        func() []trace.Link
}

const maxContexts = 1000

func makeBenchContexts(
	n int,
	bfuncs testFuncs,
) (
	r []testContext,
) {
	rnd := rand.New(rand.NewSource(101333))
	for range min(n, maxContexts) {
		var cfg trace.SpanContextConfig
		cfg.TraceID = bfuncs.parentid(rnd)
		rnd.Read(cfg.SpanID[:])
		if bfuncs.sampled() {
			cfg.TraceFlags = 3
		}
		if bfuncs.remote() {
			cfg.Remote = true
		}
		cfg.TraceState = bfuncs.tracestate()
		ctx := trace.ContextWithSpanContext(
			context.Background(),
			trace.NewSpanContext(cfg),
		)
		tid := cfg.TraceID
		if !tid.IsValid() {
			tid = bfuncs.traceid(rnd)
		}
		r = append(r, testContext{
			Context: ctx,
			SamplingParameters: SamplingParameters{
				ParentContext: ctx,
				TraceID:       tid,
				Name:          bfuncs.name(),
				Kind:          bfuncs.kind(),
				Attributes:    bfuncs.inAttributes,
				Links:         bfuncs.links(),
			},
		})
	}
	return
}

func defaultTestFuncs() testFuncs {
	return testFuncs{
		parentid: func(rnd *rand.Rand) (tid trace.TraceID) {
			rnd.Read(tid[:])
			return
		},
		traceid: func(rnd *rand.Rand) (tid trace.TraceID) {
			rnd.Read(tid[:])
			return
		},
		sampled: func() bool { return true },
		remote:  func() bool { return true },
		tracestate: func() trace.TraceState {
			ts, _ := trace.ParseTraceState("")
			return ts
		},
		name:         func() string { return "test" },
		kind:         func() trace.SpanKind { return trace.SpanKindInternal },
		inAttributes: testAttrs,
		links:        func() []trace.Link { return nil },
	}
}

func makeSimpleContexts(n int) []testContext {
	return makeBenchContexts(n, defaultTestFuncs())
}

func makeTestContext(bf testFuncs) testContext {
	return makeBenchContexts(1, bf)[0]
}

func BenchmarkAlwaysOn(b *testing.B) {
	ctxs := makeSimpleContexts(b.N)
	sampler := AlwaysSample()
	b.ResetTimer()
	for i := range b.N {
		_ = sampler.ShouldSample(ctxs[i%maxContexts].SamplingParameters)
	}
}

func BenchmarkConsistentAlwaysOn(b *testing.B) {
	ctxs := makeSimpleContexts(b.N)
	sampler := CompositeSampler(ComposableAlwaysSample())
	b.ResetTimer()
	for i := range b.N {
		_ = sampler.ShouldSample(ctxs[i%maxContexts].SamplingParameters)
	}
}

func BenchmarkComposableParentBasedUnknownThreshold(b *testing.B) {
	ctxs := makeSimpleContexts(b.N)
	sampler := CompositeSampler(ComposableParentBased(ComposableAlwaysSample()))
	b.ResetTimer()
	for i := range b.N {
		_ = sampler.ShouldSample(ctxs[i%maxContexts].SamplingParameters)
	}
}

func BenchmarkComposableParentBasedParentThreshold(b *testing.B) {
	ts, err := trace.ParseTraceState("ot=th:0")
	require.NoError(b, err)
	bfuncs := defaultTestFuncs()
	bfuncs.tracestate = func() trace.TraceState {
		return ts
	}
	ctxs := makeBenchContexts(b.N, bfuncs)
	sampler := CompositeSampler(ComposableParentBased(ComposableAlwaysSample()))
	b.ResetTimer()
	for i := range b.N {
		_ = sampler.ShouldSample(ctxs[i%maxContexts].SamplingParameters)
	}
}

func BenchmarkComposableParentBasedNonEmptyTraceStateUnknownThreshold(b *testing.B) {
	ts, err := trace.ParseTraceState("co=whateverr,ed=nowaysir")
	require.NoError(b, err)
	bfs := defaultTestFuncs()
	bfs.tracestate = func() trace.TraceState {
		return ts
	}
	ctxs := makeBenchContexts(b.N, bfs)
	sampler := CompositeSampler(ComposableParentBased(ComposableAlwaysSample()))
	b.ResetTimer()
	for i := range b.N {
		_ = sampler.ShouldSample(ctxs[i%maxContexts].SamplingParameters)
	}
}

func BenchmarkComposableParentBasedNonEmptyOTelTraceStateUnknownThreshold(b *testing.B) {
	ts, err := trace.ParseTraceState("co=whateverr,ed=nowaysir,ot=xx:abc;yy:def")
	require.NoError(b, err)
	bfs := defaultTestFuncs()
	bfs.tracestate = func() trace.TraceState {
		return ts
	}
	ctxs := makeBenchContexts(b.N, bfs)
	sampler := CompositeSampler(ComposableParentBased(ComposableAlwaysSample()))
	b.ResetTimer()
	for i := range b.N {
		_ = sampler.ShouldSample(ctxs[i%maxContexts].SamplingParameters)
	}
}

func BenchmarkComposableParentBasedNonEmptyOTelTraceStateParentThreshold(b *testing.B) {
	ts, err := trace.ParseTraceState("co=whateverr,ed=nowaysir,ot=xx:abc;yy:def;th:0")
	require.NoError(b, err)
	bfs := defaultTestFuncs()
	bfs.tracestate = func() trace.TraceState {
		return ts
	}
	ctxs := makeBenchContexts(b.N, bfs)
	sampler := CompositeSampler(ComposableParentBased(ComposableAlwaysSample()))
	b.ResetTimer()
	for i := range b.N {
		_ = sampler.ShouldSample(ctxs[i%maxContexts].SamplingParameters)
	}
}

func BenchmarkComposableParentBasedWithNonEmptyOTelTraceStateIncludingRandomness(b *testing.B) {
	ts, err := trace.ParseTraceState("co=whateverr,ed=nowaysir,ot=xx:abc;yy:def;th:0;rv:abcdefabcdefab")
	require.NoError(b, err)
	bfs := defaultTestFuncs()
	bfs.tracestate = func() trace.TraceState {
		return ts
	}
	ctxs := makeBenchContexts(b.N, bfs)
	sampler := CompositeSampler(ComposableParentBased(ComposableAlwaysSample()))
	b.ResetTimer()
	for i := range b.N {
		_ = sampler.ShouldSample(ctxs[i%maxContexts].SamplingParameters)
	}
}

func BenchmarkParentBasedNoTraceState(b *testing.B) {
	bfs := defaultTestFuncs()
	bfs.tracestate = func() trace.TraceState {
		return trace.TraceState{}
	}
	ctxs := makeBenchContexts(b.N, bfs)
	sampler := ParentBased(AlwaysSample())
	b.ResetTimer()
	for i := range b.N {
		_ = sampler.ShouldSample(ctxs[i%maxContexts].SamplingParameters)
	}
}

func BenchmarkParentBasedWithOTelTraceStateIncludingRandomness(b *testing.B) {
	ts, err := trace.ParseTraceState("co=whateverr,ed=nowaysir,ot=xx:abc;yy:def;rv:abcdefabcdefab")
	require.NoError(b, err)
	bfs := defaultTestFuncs()
	bfs.tracestate = func() trace.TraceState {
		return ts
	}
	ctxs := makeBenchContexts(b.N, bfs)
	sampler := ParentBased(AlwaysSample())
	b.ResetTimer()
	for i := range b.N {
		_ = sampler.ShouldSample(ctxs[i%maxContexts].SamplingParameters)
	}
}
