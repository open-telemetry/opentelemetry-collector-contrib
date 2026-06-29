// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package genainormalizerprocessor

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/metadata"
)

// BenchmarkProcessorBuiltIn measures the cost of normalizing spans with
// the built-in source mapping tables. Sweeps spans-per-batch across the
// three built-in source mixes.
func BenchmarkProcessorBuiltIn(b *testing.B) {
	cases := []struct {
		name    string
		sources []Source
		traces  ptrace.Traces
	}{
		{"openinference/1_span", []Source{{Name: SourceOpenInference}}, openInferenceTraces(1)},
		{"openinference/10_spans", []Source{{Name: SourceOpenInference}}, openInferenceTraces(10)},
		{"openinference/100_spans", []Source{{Name: SourceOpenInference}}, openInferenceTraces(100)},
		{"openinference/1000_spans", []Source{{Name: SourceOpenInference}}, openInferenceTraces(1000)},

		{"openllmetry/1_span", []Source{{Name: SourceOpenLLMetry}}, openLLMetryTraces(1)},
		{"openllmetry/100_spans", []Source{{Name: SourceOpenLLMetry}}, openLLMetryTraces(100)},
		{"openllmetry/1000_spans", []Source{{Name: SourceOpenLLMetry}}, openLLMetryTraces(1000)},

		{"both/100_spans", []Source{{Name: SourceOpenInference}, {Name: SourceOpenLLMetry}}, mixedBuiltInTraces(100)},
		{"both/1000_spans", []Source{{Name: SourceOpenInference}, {Name: SourceOpenLLMetry}}, mixedBuiltInTraces(1000)},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			runProcessorBenchmark(b, c.sources, c.traces)
		})
	}
}

func BenchmarkProcessorFlattenedMessages(b *testing.B) {
	cases := []struct {
		name   string
		traces ptrace.Traces
	}{
		{"1_span_2_messages", openInferenceFlattenedTraces(1, 2)},
		{"10_spans_4_messages", openInferenceFlattenedTraces(10, 4)},
		{"100_spans_4_messages", openInferenceFlattenedTraces(100, 4)},
		{"100_spans_20_messages", openInferenceFlattenedTraces(100, 20)},
		{"1000_spans_4_messages", openInferenceFlattenedTraces(1000, 4)},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			sources := []Source{{Name: SourceOpenInference, RemoveOriginals: true}}
			runProcessorBenchmark(b, sources, c.traces)
		})
	}
}

func BenchmarkProcessorFlattenedMessagesWithToolCalls(b *testing.B) {
	cases := []struct {
		name   string
		traces ptrace.Traces
	}{
		{"100_spans", openInferenceFlattenedToolCallTraces(100)},
		{"1000_spans", openInferenceFlattenedToolCallTraces(1000)},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			sources := []Source{{Name: SourceOpenInference, RemoveOriginals: true}}
			runProcessorBenchmark(b, sources, c.traces)
		})
	}
}

// BenchmarkProcessorUserDefined sweeps the user-defined mapping-table
// size against a fixed batch of 100 spans where every span attribute
// matches a mapping rule.
func BenchmarkProcessorUserDefined(b *testing.B) {
	cases := []struct {
		name        string
		mappingSize int
	}{
		{"10_mappings", 10},
		{"100_mappings", 100},
		{"1000_mappings", 1000},
		{"10000_mappings", 10000},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			sources := []Source{{
				Name:     "vendor",
				Mappings: generateMappings(c.mappingSize),
			}}
			runProcessorBenchmark(b, sources, userDefinedTraces(100, c.mappingSize))
		})
	}
}

// BenchmarkProcessorMatchRatio holds spans and mapping size constant and
// varies the fraction of span attributes that actually match the rename
// table. Isolates the cost of the attrs.Range scan from the cost of
// renames.
func BenchmarkProcessorMatchRatio(b *testing.B) {
	const spans = 100
	const attrsPerSpan = 20
	mappings := generateMappings(attrsPerSpan)

	cases := []struct {
		name     string
		matchPct int
	}{
		{"0pct_match", 0},
		{"50pct_match", 50},
		{"100pct_match", 100},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			sources := []Source{{
				Name:     "vendor",
				Mappings: mappings,
			}}
			runProcessorBenchmark(b, sources, ratioTraces(spans, attrsPerSpan, c.matchPct))
		})
	}
}

// BenchmarkProcessorValueMappings measures the overhead of the
// value_mappings transform on top of the rename pass.
func BenchmarkProcessorValueMappings(b *testing.B) {
	mappings := map[string]string{"vendor.op": "gen_ai.operation.name"}

	b.Run("without_value_mappings", func(b *testing.B) {
		sources := []Source{{Name: "vendor", Mappings: mappings}}
		runProcessorBenchmark(b, sources, opNameTraces(100))
	})

	b.Run("with_value_mappings", func(b *testing.B) {
		sources := []Source{{
			Name:     "vendor",
			Mappings: mappings,
			ValueMappings: map[string]map[string]string{
				"gen_ai.operation.name": {
					"chat_completion": "chat",
					"text_completion": "text_completion",
					"tool_invoke":     "execute_tool",
				},
			},
		}}
		runProcessorBenchmark(b, sources, opNameTraces(100))
	})
}

// BenchmarkProcessorThroughput measures parallel saturation under a
// realistic mid-size config.
func BenchmarkProcessorThroughput(b *testing.B) {
	sources := []Source{{Name: SourceOpenInference}}
	traces := openInferenceTraces(100)

	factory := NewFactory()
	cfg := &Config{Sources: sources}
	p, err := factory.CreateTraces(b.Context(), processortest.NewNopSettings(metadata.Type), cfg, dropTracesSink{})
	require.NoError(b, err)
	require.NoError(b, p.Start(b.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(b, p.Shutdown(b.Context()))
	}()

	b.ReportAllocs()
	b.ResetTimer()
	b.SetParallelism(4)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			clone := ptrace.NewTraces()
			traces.CopyTo(clone)
			if err := p.ConsumeTraces(b.Context(), clone); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkProcessorThroughputWithMessages(b *testing.B) {
	sources := []Source{{Name: SourceOpenInference, RemoveOriginals: true}}
	traces := openInferenceFlattenedTraces(100, 4)

	factory := NewFactory()
	cfg := &Config{Sources: sources}
	p, err := factory.CreateTraces(b.Context(), processortest.NewNopSettings(metadata.Type), cfg, dropTracesSink{})
	require.NoError(b, err)
	require.NoError(b, p.Start(b.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(b, p.Shutdown(b.Context())) }()

	b.ReportAllocs()
	b.ResetTimer()
	b.SetParallelism(4)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			clone := ptrace.NewTraces()
			traces.CopyTo(clone)
			if err := p.ConsumeTraces(b.Context(), clone); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkProcessorAttrsPerSpan sweeps the attributes-per-span dimension
// against a fixed 10k-entry mapping table. attrs.Range walks every
// attribute on every span, so this is the dominant cost driver. Span
// count drops at the high end to keep total batch cost bounded.
func BenchmarkProcessorAttrsPerSpan(b *testing.B) {
	mappings := generateMappings(10000)

	cases := []struct {
		name         string
		spans        int
		attrsPerSpan int
	}{
		{"1_attr", 100, 1},
		{"10_attrs", 100, 10},
		{"100_attrs", 100, 100},
		{"1000_attrs", 100, 1000},
		{"10000_attrs", 100, 10000},
		{"100000_attrs", 1, 100000},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			sources := []Source{{
				Name:     "vendor",
				Mappings: mappings,
			}}
			runProcessorBenchmark(b, sources, ratioTraces(c.spans, c.attrsPerSpan, 100))
		})
	}
}

// BenchmarkProcessorMappingsKnee sweeps mapping-table size at a fixed,
// realistic attribute count to validate or move the largeMappingsThreshold
// warning. If the curve stays flat across the sweep, the threshold is
// conservative; if there's a knee, the threshold should track it.
func BenchmarkProcessorMappingsKnee(b *testing.B) {
	const spans = 100
	const attrsPerSpan = 50

	cases := []struct {
		name        string
		mappingSize int
	}{
		{"100_mappings", 100},
		{"500_mappings", 500},
		{"1000_mappings", 1000},
		{"2000_mappings", 2000},
		{"5000_mappings", 5000},
		{"10000_mappings", 10000},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			sources := []Source{{
				Name:     "vendor",
				Mappings: generateMappings(c.mappingSize),
			}}
			runProcessorBenchmark(b, sources, ratioTraces(spans, attrsPerSpan, 100))
		})
	}
}

// BenchmarkProcessorBreakIt drives the worst case: 1000 spans, each
// carrying 100 matching attributes, against a 10k-entry user-defined
// mapping table.
func BenchmarkProcessorBreakIt(b *testing.B) {
	const spans = 1000
	const attrsPerSpan = 100
	const mappingSize = 10000

	sources := []Source{{
		Name:     "vendor",
		Mappings: generateMappings(mappingSize),
	}}
	runProcessorBenchmark(b, sources, ratioTraces(spans, attrsPerSpan, 100))
}

// BenchmarkProcessorRemoveOriginals isolates the cost of the
// remove_originals=true path against a 50-attribute span where every
// attribute matches a rename rule.
func BenchmarkProcessorRemoveOriginals(b *testing.B) {
	const spans = 100
	const attrsPerSpan = 50

	mappings := generateMappings(attrsPerSpan)

	b.Run("remove_originals=false", func(b *testing.B) {
		sources := []Source{{
			Name:     "vendor",
			Mappings: mappings,
		}}
		runProcessorBenchmark(b, sources, ratioTraces(spans, attrsPerSpan, 100))
	})

	b.Run("remove_originals=true", func(b *testing.B) {
		sources := []Source{{
			Name:            "vendor",
			RemoveOriginals: true,
			Mappings:        mappings,
		}}
		runProcessorBenchmark(b, sources, ratioTraces(spans, attrsPerSpan, 100))
	})
}

// runProcessorBenchmark spins up the processor once and replays a clone
// of the fixture each iteration. The processor mutates input data, so a
// fresh clone is required per iteration.
func runProcessorBenchmark(b *testing.B, sources []Source, traces ptrace.Traces) {
	factory := NewFactory()
	cfg := &Config{Sources: sources}

	p, err := factory.CreateTraces(b.Context(), processortest.NewNopSettings(metadata.Type), cfg, dropTracesSink{})
	require.NoError(b, err)

	require.NoError(b, p.Start(b.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(b, p.Shutdown(b.Context()))
	}()

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		clone := ptrace.NewTraces()
		traces.CopyTo(clone)
		if err := p.ConsumeTraces(b.Context(), clone); err != nil {
			b.Fatal(err)
		}
	}
}

// openInferenceTraces builds spans carrying every attribute the
// openinference built-in source recognizes, plus a couple of unrelated
// attributes to simulate noise.
func openInferenceTraces(numSpans int) ptrace.Traces {
	td := ptrace.NewTraces()
	ss := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
	for range numSpans {
		span := ss.Spans().AppendEmpty()
		attrs := span.Attributes()
		attrs.PutInt("llm.token_count.prompt", 100)
		attrs.PutInt("llm.token_count.completion", 200)
		attrs.PutStr("llm.model_name", "claude-sonnet-4")
		attrs.PutStr("llm.provider", "anthropic")
		attrs.PutStr("openinference.span.kind", "LLM")
		attrs.PutStr("agent.name", "research-agent")
		attrs.PutStr("session.id", "sess-123")
		attrs.PutStr("noise.attr.a", "x")
		attrs.PutStr("noise.attr.b", "y")
	}
	return td
}

func openInferenceFlattenedTraces(numSpans, messagesPerSpan int) ptrace.Traces {
	td := ptrace.NewTraces()
	ss := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
	for range numSpans {
		span := ss.Spans().AppendEmpty()
		attrs := span.Attributes()
		attrs.PutStr("llm.model_name", "gpt-4")
		attrs.PutStr("openinference.span.kind", "LLM")
		for i := range messagesPerSpan {
			prefix := fmt.Sprintf("llm.input_messages.%d.message.", i)
			if i%2 == 0 {
				attrs.PutStr(prefix+"role", "user")
				attrs.PutStr(prefix+"content", fmt.Sprintf("message %d", i))
			} else {
				attrs.PutStr(prefix+"role", "assistant")
				attrs.PutStr(prefix+"content", fmt.Sprintf("response %d", i))
			}
		}
		attrs.PutStr("noise.attr.a", "x")
	}
	return td
}

func openInferenceFlattenedToolCallTraces(numSpans int) ptrace.Traces {
	td := ptrace.NewTraces()
	ss := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
	for range numSpans {
		span := ss.Spans().AppendEmpty()
		attrs := span.Attributes()
		attrs.PutStr("llm.model_name", "gpt-4")
		attrs.PutStr("openinference.span.kind", "LLM")
		attrs.PutStr("llm.input_messages.0.message.role", "user")
		attrs.PutStr("llm.input_messages.0.message.content", "What is the weather?")
		attrs.PutStr("llm.output_messages.0.message.role", "assistant")
		attrs.PutStr("llm.output_messages.0.message.tool_calls.0.tool_call.id", "call_1")
		attrs.PutStr("llm.output_messages.0.message.tool_calls.0.tool_call.function.name", "get_weather")
		attrs.PutStr("llm.output_messages.0.message.tool_calls.0.tool_call.function.arguments", `{"city":"Berlin"}`)
		attrs.PutStr("llm.output_messages.0.message.tool_calls.1.tool_call.id", "call_2")
		attrs.PutStr("llm.output_messages.0.message.tool_calls.1.tool_call.function.name", "get_time")
		attrs.PutStr("llm.output_messages.0.message.tool_calls.1.tool_call.function.arguments", `{"tz":"CET"}`)
		attrs.PutStr("llm.input_messages.1.message.content", "sunny 22C")
		attrs.PutStr("llm.input_messages.1.message.tool_call_id", "call_1")
	}
	return td
}

// openLLMetryTraces builds spans carrying common openllmetry attributes
// plus noise.
func openLLMetryTraces(numSpans int) ptrace.Traces {
	td := ptrace.NewTraces()
	ss := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
	for range numSpans {
		span := ss.Spans().AppendEmpty()
		attrs := span.Attributes()
		attrs.PutInt("llm.usage.prompt_tokens", 100)
		attrs.PutInt("llm.usage.completion_tokens", 200)
		attrs.PutStr("llm.request.model", "claude-sonnet-4")
		attrs.PutStr("llm.response.model", "claude-sonnet-4")
		attrs.PutDouble("llm.request.temperature", 0.7)
		attrs.PutStr("llm.request.type", "chat")
		attrs.PutStr("traceloop.entity.name", "research-agent")
		attrs.PutStr("noise.attr.a", "x")
		attrs.PutStr("noise.attr.b", "y")
	}
	return td
}

// mixedBuiltInTraces builds spans carrying attributes for both built-in
// sources, simulating the worst case where both pipelines fire.
func mixedBuiltInTraces(numSpans int) ptrace.Traces {
	td := ptrace.NewTraces()
	ss := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
	for range numSpans {
		span := ss.Spans().AppendEmpty()
		attrs := span.Attributes()
		// openinference attrs
		attrs.PutStr("llm.model_name", "claude-sonnet-4")
		attrs.PutStr("openinference.span.kind", "LLM")
		// openllmetry attrs
		attrs.PutStr("llm.request.model", "claude-sonnet-4")
		attrs.PutStr("llm.request.type", "chat")
	}
	return td
}

// userDefinedTraces builds spans carrying mappingSize-many user-defined
// attributes that all match the generated mapping table.
func userDefinedTraces(numSpans, mappingSize int) ptrace.Traces {
	td := ptrace.NewTraces()
	ss := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
	for range numSpans {
		span := ss.Spans().AppendEmpty()
		attrs := span.Attributes()
		for i := range mappingSize {
			attrs.PutStr(fmt.Sprintf("vendor.attr.%d", i), "value")
		}
	}
	return td
}

// ratioTraces builds spans carrying attrsPerSpan attributes; matchPct of
// them match the user-defined mapping table (vendor.attr.N), the rest are
// non-matching noise.
func ratioTraces(numSpans, attrsPerSpan, matchPct int) ptrace.Traces {
	td := ptrace.NewTraces()
	ss := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
	matchCount := attrsPerSpan * matchPct / 100
	for range numSpans {
		span := ss.Spans().AppendEmpty()
		attrs := span.Attributes()
		for i := range matchCount {
			attrs.PutStr(fmt.Sprintf("vendor.attr.%d", i), "value")
		}
		for i := matchCount; i < attrsPerSpan; i++ {
			attrs.PutStr(fmt.Sprintf("noise.%d", i), "value")
		}
	}
	return td
}

// opNameTraces builds spans carrying a single attribute that renames to
// gen_ai.operation.name, used to isolate value_mappings overhead.
func opNameTraces(numSpans int) ptrace.Traces {
	td := ptrace.NewTraces()
	ss := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
	for range numSpans {
		span := ss.Spans().AppendEmpty()
		span.Attributes().PutStr("vendor.op", "chat_completion")
	}
	return td
}

// generateMappings produces a synthetic user-defined rename table of the
// requested size, with keys vendor.attr.0..N-1 mapping to gen_ai.user.N.
func generateMappings(size int) map[string]string {
	m := make(map[string]string, size)
	for i := range size {
		m[fmt.Sprintf("vendor.attr.%d", i)] = fmt.Sprintf("gen_ai.user.%d", i)
	}
	return m
}

type dropTracesSink struct{}

func (dropTracesSink) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (dropTracesSink) ConsumeTraces(context.Context, ptrace.Traces) error {
	return nil
}
