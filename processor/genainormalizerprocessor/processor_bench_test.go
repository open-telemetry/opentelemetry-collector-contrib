// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package genainormalizerprocessor

import (
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func bothSourcesProcessor() *genaiNormalizerProcessor {
	return newGenaiNormalizerProcessor(&Config{
		Sources: map[SourceName]Source{
			SourceOpenInference: {RemoveOriginals: true},
			SourceOpenLLMetry:   {RemoveOriginals: true},
		},
	})
}

// BenchmarkNormalizeOpenInference benchmarks normalization of a typical
// OpenInference LLM span with attributes that need mapping.
func BenchmarkNormalizeOpenInference(b *testing.B) {
	p := bothSourcesProcessor()
	template := pcommon.NewMap()
	template.PutStr("openinference.span.kind", "LLM")
	template.PutInt("llm.token_count.prompt", 150)
	template.PutInt("llm.token_count.completion", 50)
	template.PutStr("llm.model_name", "claude-sonnet-4-20250514")
	template.PutStr("llm.provider", "anthropic")
	template.PutStr("session.id", "sess-123")
	template.PutStr("agent.name", "research-agent")
	template.PutStr("service.name", "my-service")
	template.PutStr("http.method", "POST")

	attrs := pcommon.NewMap()
	b.ResetTimer()
	for range b.N {
		attrs.Clear()
		template.CopyTo(attrs)
		for s := range p.sources {
			p.sources[s].normalizeAttributes(attrs)
		}
	}
}

// BenchmarkNormalizeNoMatch benchmarks the no-op case where a span
// has no GenAI attributes (early exit path).
func BenchmarkNormalizeNoMatch(b *testing.B) {
	p := bothSourcesProcessor()
	template := pcommon.NewMap()
	template.PutStr("service.name", "my-service")
	template.PutStr("http.method", "POST")
	template.PutStr("http.url", "https://api.example.com")
	template.PutInt("http.status_code", 200)

	attrs := pcommon.NewMap()
	b.ResetTimer()
	for range b.N {
		attrs.Clear()
		template.CopyTo(attrs)
		for s := range p.sources {
			p.sources[s].normalizeAttributes(attrs)
		}
	}
}

// BenchmarkNormalizeBatch100 benchmarks processing a batch of 100 spans.
// Traces are rebuilt each iteration so measurements do not drift into the
// no-match fast path after the first iteration normalizes (and, with
// RemoveOriginals=true, strips) the source attributes.
func BenchmarkNormalizeBatch100(b *testing.B) {
	p := bothSourcesProcessor()
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		td := buildBatchTraces(100)
		b.StartTimer()
		_, _ = p.processTraces(b.Context(), td)
	}
}

func buildBatchTraces(n int) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()
	for range n {
		span := ils.Spans().AppendEmpty()
		span.SetName("test-span")
		attrs := span.Attributes()
		attrs.PutStr("openinference.span.kind", "LLM")
		attrs.PutInt("llm.token_count.prompt", 150)
		attrs.PutInt("llm.token_count.completion", 50)
		attrs.PutStr("llm.model_name", "claude-sonnet-4-20250514")
		attrs.PutStr("llm.provider", "anthropic")
		attrs.PutStr("session.id", "sess-123")
		attrs.PutStr("service.name", "my-service")
	}
	return td
}
