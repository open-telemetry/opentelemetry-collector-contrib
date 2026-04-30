// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package genainormalizerprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/metadata"
)

func singleSource(name SourceName, src Source) *Config {
	return &Config{Sources: map[SourceName]Source{name: src}}
}

func runProcessor(t *testing.T, cfg *Config, input ptrace.Traces) ptrace.Traces {
	t.Helper()
	sink := new(consumertest.TracesSink)
	p, err := createTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, p.ConsumeTraces(t.Context(), input))
	require.Len(t, sink.AllTraces(), 1)
	return sink.AllTraces()[0]
}

func newSpan() (ptrace.Traces, ptrace.Span) {
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	return td, span
}

func spanAttrs(out ptrace.Traces) pcommon.Map {
	return out.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()
}

func TestNormalize_SpanAttributes(t *testing.T) {
	tests := []struct {
		name   string
		cfg    *Config
		setup  func(pcommon.Map)
		verify func(*testing.T, pcommon.Map)
	}{
		{
			name: "openinference: token counts and model, remove_originals=true",
			cfg:  singleSource(SourceOpenInference, Source{RemoveOriginals: true}),
			setup: func(attrs pcommon.Map) {
				attrs.PutInt("llm.token_count.prompt", 100)
				attrs.PutInt("llm.token_count.completion", 200)
				attrs.PutStr("llm.model_name", "claude-sonnet-4")
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				assertAttrInt(t, attrs, "gen_ai.usage.input_tokens", 100)
				assertAttrInt(t, attrs, "gen_ai.usage.output_tokens", 200)
				assertAttrStr(t, attrs, "gen_ai.request.model", "claude-sonnet-4")
				_, ok := attrs.Get("llm.token_count.prompt")
				assert.False(t, ok, "expected llm.token_count.prompt to be removed")
			},
		},
		{
			name: "openllmetry: token counts and model, remove_originals=true",
			cfg:  singleSource(SourceOpenLLMetry, Source{RemoveOriginals: true}),
			setup: func(attrs pcommon.Map) {
				attrs.PutInt("llm.usage.prompt_tokens", 150)
				attrs.PutStr("llm.request.model", "gpt-4o")
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				assertAttrInt(t, attrs, "gen_ai.usage.input_tokens", 150)
				assertAttrStr(t, attrs, "gen_ai.request.model", "gpt-4o")
			},
		},
		{
			name: "openllmetry only: openinference attrs pass through",
			cfg:  singleSource(SourceOpenLLMetry, Source{RemoveOriginals: true}),
			setup: func(attrs pcommon.Map) {
				attrs.PutInt("llm.token_count.prompt", 100)
				attrs.PutStr("llm.model_name", "claude-sonnet-4")
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				assertAttrInt(t, attrs, "llm.token_count.prompt", 100)
				assertAttrStr(t, attrs, "llm.model_name", "claude-sonnet-4")
				_, ok := attrs.Get("gen_ai.usage.input_tokens")
				assert.False(t, ok, "openinference attrs must not be mapped when only openllmetry is enabled")
			},
		},
		{
			name: "openinference only: openllmetry attrs pass through",
			cfg:  singleSource(SourceOpenInference, Source{RemoveOriginals: true}),
			setup: func(attrs pcommon.Map) {
				attrs.PutInt("llm.usage.prompt_tokens", 200)
				attrs.PutStr("llm.request.model", "gpt-4o")
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				assertAttrInt(t, attrs, "llm.usage.prompt_tokens", 200)
				assertAttrStr(t, attrs, "llm.request.model", "gpt-4o")
				_, ok := attrs.Get("gen_ai.usage.input_tokens")
				assert.False(t, ok, "openllmetry attrs must not be mapped when only openinference is enabled")
			},
		},
		{
			name: "non-GenAI attrs untouched",
			cfg: &Config{Sources: map[SourceName]Source{
				SourceOpenInference: {RemoveOriginals: true},
				SourceOpenLLMetry:   {RemoveOriginals: true},
			}},
			setup: func(attrs pcommon.Map) {
				attrs.PutStr("http.method", "GET")
				attrs.PutInt("http.status_code", 200)
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				assert.Equal(t, 2, attrs.Len())
				assertAttrStr(t, attrs, "http.method", "GET")
				assertAttrInt(t, attrs, "http.status_code", 200)
			},
		},
		{
			name: "remove_originals=false keeps source",
			cfg:  singleSource(SourceOpenInference, Source{RemoveOriginals: false}),
			setup: func(attrs pcommon.Map) {
				attrs.PutInt("llm.token_count.prompt", 100)
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				assertAttrInt(t, attrs, "gen_ai.usage.input_tokens", 100)
				assertAttrInt(t, attrs, "llm.token_count.prompt", 100)
			},
		},
		{
			name: "overwrite=false skips existing target",
			cfg:  singleSource(SourceOpenInference, Source{RemoveOriginals: true, Overwrite: false}),
			setup: func(attrs pcommon.Map) {
				attrs.PutStr("llm.model_name", "new-model")
				attrs.PutStr("gen_ai.request.model", "existing-model")
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				assertAttrStr(t, attrs, "gen_ai.request.model", "existing-model")
			},
		},
		{
			name: "overwrite=true replaces existing target",
			cfg:  singleSource(SourceOpenInference, Source{RemoveOriginals: true, Overwrite: true}),
			setup: func(attrs pcommon.Map) {
				attrs.PutStr("llm.model_name", "new-model")
				attrs.PutStr("gen_ai.request.model", "existing-model")
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				assertAttrStr(t, attrs, "gen_ai.request.model", "new-model")
			},
		},
		{
			name: "wrapSlice: finish_reason string to finish_reasons slice",
			cfg:  singleSource(SourceOpenLLMetry, Source{RemoveOriginals: true}),
			setup: func(attrs pcommon.Map) {
				attrs.PutStr("llm.response.finish_reason", "stop")
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				v, ok := attrs.Get("gen_ai.response.finish_reasons")
				require.True(t, ok, "missing gen_ai.response.finish_reasons")
				require.Equal(t, pcommon.ValueTypeSlice, v.Type())
				require.Equal(t, 1, v.Slice().Len())
				assert.Equal(t, "stop", v.Slice().At(0).Str())
			},
		},
		{
			name: "value mapping: openinference.span.kind=LLM to gen_ai.operation.name=chat",
			cfg:  singleSource(SourceOpenInference, Source{RemoveOriginals: true}),
			setup: func(attrs pcommon.Map) {
				attrs.PutStr("openinference.span.kind", "LLM")
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				assertAttrStr(t, attrs, "gen_ai.operation.name", "chat")
			},
		},
		{
			name: "custom mappings on built-in source (additive)",
			cfg: singleSource(SourceOpenInference, Source{
				RemoveOriginals: true,
				CustomMappings:  map[string]string{"my_vendor.model": "gen_ai.request.model"},
			}),
			setup: func(attrs pcommon.Map) {
				attrs.PutStr("my_vendor.model", "custom-model-v1")
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				assertAttrStr(t, attrs, "gen_ai.request.model", "custom-model-v1")
				_, ok := attrs.Get("my_vendor.model")
				assert.False(t, ok)
			},
		},
		{
			name: "custom mapping overrides built-in on conflict",
			cfg: singleSource(SourceOpenInference, Source{
				RemoveOriginals: true,
				CustomMappings:  map[string]string{"llm.model_name": "gen_ai.response.model"},
			}),
			setup: func(attrs pcommon.Map) {
				attrs.PutStr("llm.model_name", "gpt-4o")
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				assertAttrStr(t, attrs, "gen_ai.response.model", "gpt-4o")
				_, ok := attrs.Get("gen_ai.request.model")
				assert.False(t, ok, "built-in mapping should have been overridden")
			},
		},
		{
			name: "custom source only (no built-in mappings applied)",
			cfg: singleSource(SourceCustom, Source{
				RemoveOriginals: true,
				CustomMappings:  map[string]string{"my_vendor.model": "gen_ai.request.model"},
			}),
			setup: func(attrs pcommon.Map) {
				attrs.PutStr("my_vendor.model", "custom-model-v1")
				attrs.PutStr("llm.model_name", "should-not-map")
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				assertAttrStr(t, attrs, "gen_ai.request.model", "custom-model-v1")
				assertAttrStr(t, attrs, "llm.model_name", "should-not-map")
			},
		},
		{
			name: "custom source: double value passes through",
			cfg: singleSource(SourceCustom, Source{
				RemoveOriginals: true,
				CustomMappings:  map[string]string{"my_vendor.temperature": "gen_ai.request.temperature"},
			}),
			setup: func(attrs pcommon.Map) {
				attrs.PutDouble("my_vendor.temperature", 0.7)
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				v, ok := attrs.Get("gen_ai.request.temperature")
				require.True(t, ok)
				assert.Equal(t, 0.7, v.Double())
			},
		},
		{
			name: "custom source: bool value passes through",
			cfg: singleSource(SourceCustom, Source{
				RemoveOriginals: true,
				CustomMappings:  map[string]string{"my_vendor.streaming": "gen_ai.request.streaming"},
			}),
			setup: func(attrs pcommon.Map) {
				attrs.PutBool("my_vendor.streaming", true)
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				v, ok := attrs.Get("gen_ai.request.streaming")
				require.True(t, ok)
				assert.True(t, v.Bool())
			},
		},
		{
			name: "wrapSlice fallback when source value is already a slice",
			cfg:  singleSource(SourceOpenLLMetry, Source{RemoveOriginals: true}),
			setup: func(attrs pcommon.Map) {
				s := attrs.PutEmptySlice("llm.response.finish_reason")
				s.AppendEmpty().SetStr("stop")
				s.AppendEmpty().SetStr("length")
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				v, ok := attrs.Get("gen_ai.response.finish_reasons")
				require.True(t, ok)
				require.Equal(t, pcommon.ValueTypeSlice, v.Type())
				require.Equal(t, 2, v.Slice().Len())
				assert.Equal(t, "stop", v.Slice().At(0).Str())
				assert.Equal(t, "length", v.Slice().At(1).Str())
			},
		},
		{
			name: "multiple sources: both applied on one span",
			cfg: &Config{Sources: map[SourceName]Source{
				SourceOpenInference: {RemoveOriginals: true},
				SourceOpenLLMetry:   {RemoveOriginals: true},
			}},
			setup: func(attrs pcommon.Map) {
				attrs.PutInt("llm.token_count.prompt", 100)         // openinference
				attrs.PutInt("llm.usage.completion_tokens", 50)     // openllmetry
				attrs.PutStr("llm.model_name", "claude-sonnet-4")   // openinference
				attrs.PutStr("llm.response.model", "gpt-4o-output") // openllmetry
			},
			verify: func(t *testing.T, attrs pcommon.Map) {
				assertAttrInt(t, attrs, "gen_ai.usage.input_tokens", 100)
				assertAttrInt(t, attrs, "gen_ai.usage.output_tokens", 50)
				assertAttrStr(t, attrs, "gen_ai.request.model", "claude-sonnet-4")
				assertAttrStr(t, attrs, "gen_ai.response.model", "gpt-4o-output")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td, span := newSpan()
			tt.setup(span.Attributes())
			out := runProcessor(t, tt.cfg, td)
			tt.verify(t, spanAttrs(out))
		})
	}
}

func TestNormalize_SpanEvents(t *testing.T) {
	tests := []struct {
		name   string
		cfg    *Config
		setup  func(ptrace.Span)
		verify func(*testing.T, ptrace.Span)
	}{
		{
			name: "openinference tool_call event: tool.name and arguments",
			cfg:  singleSource(SourceOpenInference, Source{RemoveOriginals: true}),
			setup: func(span ptrace.Span) {
				span.Attributes().PutStr("llm.model_name", "claude-sonnet-4")
				evt := span.Events().AppendEmpty()
				evt.SetName("tool_call")
				evt.Attributes().PutStr("tool.name", "search")
				evt.Attributes().PutStr("tool_call.function.arguments", `{"q":"test"}`)
			},
			verify: func(t *testing.T, span ptrace.Span) {
				assertAttrStr(t, span.Attributes(), "gen_ai.request.model", "claude-sonnet-4")
				evtAttrs := span.Events().At(0).Attributes()
				assertAttrStr(t, evtAttrs, "gen_ai.tool.name", "search")
				assertAttrStr(t, evtAttrs, "gen_ai.tool.call.arguments", `{"q":"test"}`)
				_, ok := evtAttrs.Get("tool.name")
				assert.False(t, ok)
			},
		},
		{
			name: "openllmetry tool_call event: definitions and finish_reason wrap",
			cfg:  singleSource(SourceOpenLLMetry, Source{RemoveOriginals: true}),
			setup: func(span ptrace.Span) {
				span.Attributes().PutStr("llm.request.model", "gpt-4o")
				span.Attributes().PutStr("llm.request.type", "chat")
				evt := span.Events().AppendEmpty()
				evt.SetName("tool_call")
				evt.Attributes().PutStr("llm.request.functions", `[{"name":"get_weather"}]`)
				evt.Attributes().PutStr("llm.response.finish_reason", "tool_calls")
			},
			verify: func(t *testing.T, span ptrace.Span) {
				assertAttrStr(t, span.Attributes(), "gen_ai.request.model", "gpt-4o")
				assertAttrStr(t, span.Attributes(), "gen_ai.operation.name", "chat")
				evtAttrs := span.Events().At(0).Attributes()
				assertAttrStr(t, evtAttrs, "gen_ai.tool.definitions", `[{"name":"get_weather"}]`)
				v, ok := evtAttrs.Get("gen_ai.response.finish_reasons")
				require.True(t, ok)
				require.Equal(t, 1, v.Slice().Len())
				assert.Equal(t, "tool_calls", v.Slice().At(0).Str())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td, span := newSpan()
			tt.setup(span)
			out := runProcessor(t, tt.cfg, td)
			tt.verify(t, out.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0))
		})
	}
}

func TestCreateTracesProcessor_RejectsInvalidConfig(t *testing.T) {
	_, err := createTracesProcessor(
		t.Context(),
		processortest.NewNopSettings(metadata.Type),
		singleSource("bogus", Source{}),
		new(consumertest.TracesSink),
	)
	require.Error(t, err)
}

func assertAttrInt(t *testing.T, attrs pcommon.Map, key string, want int64) {
	t.Helper()
	v, ok := attrs.Get(key)
	require.True(t, ok, "missing attribute %s", key)
	assert.Equal(t, want, v.Int(), key)
}

func assertAttrStr(t *testing.T, attrs pcommon.Map, key, want string) {
	t.Helper()
	v, ok := attrs.Get(key)
	require.True(t, ok, "missing attribute %s", key)
	assert.Equal(t, want, v.Str(), key)
}
