package genainormalizerprocessor

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// BenchmarkNormalizeOpenInference benchmarks normalization of a typical
// OpenInference LLM span with 8 attributes that need mapping.
func BenchmarkNormalizeOpenInference(b *testing.B) {
	p := &genaiNormalizerProcessor{
		lookupTable: BuildLookupTable([]string{"openinference", "openllmetry"}),
		removeOrig:  true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		attrs := pcommon.NewMap()
		attrs.PutStr("openinference.span.kind", "LLM")
		attrs.PutInt("llm.token_count.prompt", 150)
		attrs.PutInt("llm.token_count.completion", 50)
		attrs.PutStr("llm.model_name", "claude-sonnet-4-20250514")
		attrs.PutStr("llm.provider", "anthropic")
		attrs.PutStr("session.id", "sess-123")
		attrs.PutStr("agent.name", "research-agent")
		// Non-GenAI attributes that should be skipped
		attrs.PutStr("service.name", "my-service")
		attrs.PutStr("http.method", "POST")
		p.normalizeAttributes(attrs)
	}
}

// BenchmarkNormalizeNoMatch benchmarks the no-op case where a span
// has no GenAI attributes (early exit path).
func BenchmarkNormalizeNoMatch(b *testing.B) {
	p := &genaiNormalizerProcessor{
		lookupTable: BuildLookupTable([]string{"openinference", "openllmetry"}),
		removeOrig:  true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		attrs := pcommon.NewMap()
		attrs.PutStr("service.name", "my-service")
		attrs.PutStr("http.method", "POST")
		attrs.PutStr("http.url", "https://api.example.com")
		attrs.PutInt("http.status_code", 200)
		p.normalizeAttributes(attrs)
	}
}

// BenchmarkNormalizeBatch benchmarks processing a batch of 100 spans,
// simulating a realistic collector workload.
func BenchmarkNormalizeBatch100(b *testing.B) {
	p := &genaiNormalizerProcessor{
		lookupTable: BuildLookupTable([]string{"openinference", "openllmetry"}),
		removeOrig:  true,
	}

	td := buildBatchTraces(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = p.processTraces(context.Background(), td)
	}
}

func buildBatchTraces(n int) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()
	for i := 0; i < n; i++ {
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
