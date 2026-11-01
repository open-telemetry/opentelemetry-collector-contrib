// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package tracebench

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"runtime"
	"runtime/pprof"
	"syscall"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

func processCPU(tb testing.TB) int64 {
	tb.Helper()
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		tb.Fatal(err)
	}
	return usage.Utime.Nano() + usage.Stime.Nano()
}

func BenchmarkTracePipeline(b *testing.B) {
	spec, err := ReadSpec(os.Getenv("TRACEBENCH_SPEC"))
	if err != nil {
		b.Fatal(err)
	}
	bank, err := MakeBank(spec)
	if err != nil {
		b.Fatal(err)
	}
	pipeline, err := NewPipeline(b.Context(), spec)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := pipeline.Close(context.Background()); err != nil {
			b.Error(err)
		}
	})
	if path := os.Getenv("TRACEBENCH_CPU_PROFILE"); path != "" {
		file, err := os.Create(path)
		if err != nil {
			b.Fatal(err)
		}
		if err := pprof.StartCPUProfile(file); err != nil {
			file.Close()
			b.Fatal(err)
		}
		b.Cleanup(func() {
			pprof.StopCPUProfile()
			if err := file.Close(); err != nil {
				b.Error(err)
			}
		})
	}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	cpuStart := processCPU(b)
	var spans, wireBytes int64
	var decodeElapsed time.Duration
	index := 0
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		request := bank[index%len(bank)]
		index++
		start := time.Now()
		data, err := request.Decode()
		decodeElapsed += time.Since(start)
		if err != nil {
			b.Fatal(err)
		}
		if err := pipeline.Consume(b.Context(), data); err != nil {
			b.Fatal(err)
		}
		spans += int64(request.Spans)
		wireBytes += int64(len(request.Wire))
	}
	b.StopTimer()
	cpuEnd := processCPU(b)
	runtime.ReadMemStats(&after)
	b.ReportMetric(float64(spans)/float64(b.N), "spans/op")
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(spans), "ns/span")
	b.ReportMetric(float64(cpuEnd-cpuStart)/float64(spans), "cpu-ns/span")
	b.ReportMetric(float64(after.TotalAlloc-before.TotalAlloc)/float64(spans), "allocated-B/span")
	b.ReportMetric(float64(wireBytes)/float64(spans), "wire-B/span")
	b.ReportMetric(float64(decodeElapsed.Nanoseconds())/float64(spans), "ns/span-decode")
	b.ReportMetric(float64(pipeline.Output)/float64(spans), "survival")
	for _, counter := range pipeline.Counters() {
		b.ReportMetric(float64(counter.Elapsed.Nanoseconds())/float64(spans), "ns/span-"+counter.Name)
	}
}

func TestTraceTopology(t *testing.T) {
	s := DefaultSpec()
	s.TraceSpans = 4
	s.Batches[0].Size = 9
	s.Classes[0].Resources = 9
	bank, err := MakeBank(s)
	if err != nil {
		t.Fatal(err)
	}
	data, err := bank[0].Decode()
	if err != nil {
		t.Fatal(err)
	}
	spans := data.ResourceSpans().At(0).ScopeSpans().At(0).Spans()
	for index := range spans.Len() {
		span := spans.At(index)
		root := spans.At(index / s.TraceSpans * s.TraceSpans)
		if span.TraceID() != root.TraceID() {
			t.Fatal("trace group split")
		}
		if index%s.TraceSpans == 0 {
			if !span.ParentSpanID().IsEmpty() {
				t.Fatal("root has a parent")
			}
			if index > 0 && span.TraceID() == spans.At(index-1).TraceID() {
				t.Fatal("adjacent traces share an ID")
			}
		} else if span.ParentSpanID() != root.SpanID() {
			t.Fatal("child does not reference its root")
		}
	}
}

func TestWorkloadCodecs(t *testing.T) {
	for _, format := range []string{"proto", "json"} {
		for _, compressed := range []bool{false, true} {
			s := DefaultSpec()
			s.Batches[0].Format, s.Batches[0].Gzip = format, compressed
			bank, err := MakeBank(s)
			if err != nil {
				t.Fatal(err)
			}
			data, err := bank[0].Decode()
			if err != nil {
				t.Fatal(err)
			}
			if data.SpanCount() != s.Batches[0].Size {
				t.Fatalf("decoded %d spans", data.SpanCount())
			}
			if data.ResourceSpans().Len() != 16 {
				t.Fatalf("unexpected resource grouping: %d", data.ResourceSpans().Len())
			}
		}
	}
}

func TestPipelineRewriteAndSampling(t *testing.T) {
	for _, rate := range []float64{0, 100} {
		s := DefaultSpec()
		s.Stats = false
		s.SamplePercent = rate
		bank, err := MakeBank(s)
		if err != nil {
			t.Fatal(err)
		}
		p, err := NewPipeline(t.Context(), s)
		if err != nil {
			t.Fatal(err)
		}
		data, err := bank[0].Decode()
		if err != nil {
			t.Fatal(err)
		}
		if err := p.Consume(t.Context(), data); err != nil {
			t.Fatal(err)
		}
		if rate == 0 && p.Output != 0 || rate == 100 && p.Output != int64(bank[0].Spans) {
			t.Fatalf("incorrect sampling output: %d", p.Output)
		}
		if rate == 100 {
			attributes := data.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()
			if _, found := attributes.Get("old.field"); found {
				t.Fatal("original key remains")
			}
			if value, found := attributes.Get("new.field"); !found || value.Str() != "example" {
				t.Fatal("replacement key/value missing")
			}
		}
		if err := p.Close(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestInvalidWorkloads(t *testing.T) {
	for _, mutate := range []func(*Spec){
		func(s *Spec) { s.Bank = 0 },
		func(s *Spec) { s.TraceSpans = 0 },
		func(s *Spec) { s.Classes[0].Weight = math.NaN() },
		func(s *Spec) { s.Batches[0].Size = 0 },
		func(s *Spec) { s.Batches[0].Format = "invalid" },
		func(s *Spec) { s.SamplePercent = 101 },
		func(s *Spec) { s.Common[0].File = "conflicting.yaml" },
	} {
		s := DefaultSpec()
		mutate(&s)
		if err := s.Validate(); err == nil {
			t.Fatal("invalid workload accepted")
		}
	}
	s := DefaultSpec()
	s.Common[0].Type = "invalid"
	if p, err := NewPipeline(t.Context(), s); err == nil {
		p.Close(t.Context())
		t.Fatal("unknown processor accepted")
	}
	if _, err := (Request{Wire: []byte("invalid"), Gzip: true}).Decode(); err == nil {
		t.Fatal("invalid gzip accepted")
	}
}

func TestConfiguredCensus(t *testing.T) {
	path := os.Getenv("TRACEBENCH_SPEC")
	s, err := ReadSpec(path)
	if err != nil {
		t.Fatal(err)
	}
	bank, err := MakeBank(s)
	if err != nil {
		t.Fatal(err)
	}
	pipeline, err := NewPipeline(t.Context(), s)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pipeline.Close(t.Context()); err != nil {
			t.Error(err)
		}
	})
	var spans, wire int64
	kinds := make(map[int32]int64)
	scopes := make(map[string]int64)
	traces := make(map[pcommon.TraceID]struct{})
	var roots int64
	resources := 0
	for _, request := range bank {
		data, err := request.Decode()
		if err != nil {
			t.Fatal(err)
		}
		spans += int64(data.SpanCount())
		wire += int64(len(request.Wire))
		resources += data.ResourceSpans().Len()
		for _, rs := range data.ResourceSpans().All() {
			for _, scope := range rs.ScopeSpans().All() {
				scopes[scope.Scope().Name()] += int64(scope.Spans().Len())
				for _, span := range scope.Spans().All() {
					kinds[int32(span.Kind())]++
					traces[span.TraceID()] = struct{}{}
					if span.ParentSpanID().IsEmpty() {
						roots++
					}
				}
			}
		}
		if err := pipeline.Consume(t.Context(), data); err != nil {
			t.Fatal(err)
		}
	}
	census := struct {
		Batches   int              `json:"batches"`
		Spans     int64            `json:"spans"`
		Wire      int64            `json:"wire_bytes"`
		Resources int              `json:"resources"`
		Kinds     map[int32]int64  `json:"kinds"`
		Scopes    map[string]int64 `json:"scopes"`
		Roots     int64            `json:"roots"`
		Traces    int              `json:"traces"`
		Survivors int64            `json:"survivors"`
		Counters  []Counter        `json:"counters"`
	}{len(bank), spans, wire, resources, kinds, scopes, roots, len(traces), pipeline.Output, pipeline.Counters()}
	result, err := json.Marshal(census)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("TRACEBENCH_CENSUS=" + string(result))
}
