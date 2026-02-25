package genainormalizerprocessor

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestNormalizeOpenInference(t *testing.T) {
	cfg := &Config{Profiles: []string{"openinference"}, RemoveOriginals: true}
	sink := new(consumertest.TracesSink)
	p, err := createTracesProcessor(context.Background(), processortest.NewNopSettings(component.MustNewType(typeStr)), cfg, sink)
	if err != nil {
		t.Fatal(err)
	}

	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutInt("llm.token_count.prompt", 100)
	span.Attributes().PutInt("llm.token_count.completion", 200)
	span.Attributes().PutStr("llm.model_name", "claude-sonnet-4")

	if err := p.ConsumeTraces(context.Background(), td); err != nil {
		t.Fatal(err)
	}

	out := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()
	assertAttrInt(t, out, "gen_ai.usage.input_tokens", 100)
	assertAttrInt(t, out, "gen_ai.usage.output_tokens", 200)
	assertAttrStr(t, out, "gen_ai.request.model", "claude-sonnet-4")

	if _, ok := out.Get("llm.token_count.prompt"); ok {
		t.Error("expected llm.token_count.prompt to be removed")
	}
}

func TestNormalizeOpenLLMetry(t *testing.T) {
	cfg := &Config{Profiles: []string{"openllmetry"}, RemoveOriginals: true}
	sink := new(consumertest.TracesSink)
	p, err := createTracesProcessor(context.Background(), processortest.NewNopSettings(component.MustNewType(typeStr)), cfg, sink)
	if err != nil {
		t.Fatal(err)
	}

	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutInt("llm.usage.prompt_tokens", 150)
	span.Attributes().PutStr("llm.request.model", "gpt-4o")

	if err := p.ConsumeTraces(context.Background(), td); err != nil {
		t.Fatal(err)
	}

	out := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()
	assertAttrInt(t, out, "gen_ai.usage.input_tokens", 150)
	assertAttrStr(t, out, "gen_ai.request.model", "gpt-4o")
}

func TestOpenLLMetryOnlyIgnoresOpenInference(t *testing.T) {
	cfg := &Config{Profiles: []string{"openllmetry"}, RemoveOriginals: true}
	sink := new(consumertest.TracesSink)
	p, err := createTracesProcessor(context.Background(), processortest.NewNopSettings(component.MustNewType(typeStr)), cfg, sink)
	if err != nil {
		t.Fatal(err)
	}

	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutInt("llm.token_count.prompt", 100)
	span.Attributes().PutStr("llm.model_name", "claude-sonnet-4")

	if err := p.ConsumeTraces(context.Background(), td); err != nil {
		t.Fatal(err)
	}

	out := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()
	assertAttrInt(t, out, "llm.token_count.prompt", 100)
	assertAttrStr(t, out, "llm.model_name", "claude-sonnet-4")
	if _, ok := out.Get("gen_ai.usage.input_tokens"); ok {
		t.Error("OpenInference attr should not be mapped when only openllmetry profile is enabled")
	}
}

func TestOpenInferenceOnlyIgnoresOpenLLMetry(t *testing.T) {
	cfg := &Config{Profiles: []string{"openinference"}, RemoveOriginals: true}
	sink := new(consumertest.TracesSink)
	p, err := createTracesProcessor(context.Background(), processortest.NewNopSettings(component.MustNewType(typeStr)), cfg, sink)
	if err != nil {
		t.Fatal(err)
	}

	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutInt("llm.usage.prompt_tokens", 200)
	span.Attributes().PutStr("llm.request.model", "gpt-4o")

	if err := p.ConsumeTraces(context.Background(), td); err != nil {
		t.Fatal(err)
	}

	out := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()
	assertAttrInt(t, out, "llm.usage.prompt_tokens", 200)
	assertAttrStr(t, out, "llm.request.model", "gpt-4o")
	if _, ok := out.Get("gen_ai.usage.input_tokens"); ok {
		t.Error("OpenLLMetry attr should not be mapped when only openinference profile is enabled")
	}
}

func TestNoOpOnNonGenAISpan(t *testing.T) {
	cfg := &Config{Profiles: []string{"openinference", "openllmetry"}, RemoveOriginals: true}
	sink := new(consumertest.TracesSink)
	p, err := createTracesProcessor(context.Background(), processortest.NewNopSettings(component.MustNewType(typeStr)), cfg, sink)
	if err != nil {
		t.Fatal(err)
	}

	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutStr("http.method", "GET")
	span.Attributes().PutInt("http.status_code", 200)

	if err := p.ConsumeTraces(context.Background(), td); err != nil {
		t.Fatal(err)
	}

	out := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()
	if out.Len() != 2 {
		t.Errorf("expected 2 attributes unchanged, got %d", out.Len())
	}
}

func TestKeepOriginals(t *testing.T) {
	cfg := &Config{Profiles: []string{"openinference"}, RemoveOriginals: false}
	sink := new(consumertest.TracesSink)
	p, err := createTracesProcessor(context.Background(), processortest.NewNopSettings(component.MustNewType(typeStr)), cfg, sink)
	if err != nil {
		t.Fatal(err)
	}

	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutInt("llm.token_count.prompt", 100)

	if err := p.ConsumeTraces(context.Background(), td); err != nil {
		t.Fatal(err)
	}

	out := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()
	assertAttrInt(t, out, "gen_ai.usage.input_tokens", 100)
	if _, ok := out.Get("llm.token_count.prompt"); !ok {
		t.Error("expected llm.token_count.prompt to be kept")
	}
}

func TestWrapSliceFinishReason(t *testing.T) {
	cfg := &Config{Profiles: []string{"openllmetry"}, RemoveOriginals: true}
	sink := new(consumertest.TracesSink)
	p, err := createTracesProcessor(context.Background(), processortest.NewNopSettings(component.MustNewType(typeStr)), cfg, sink)
	if err != nil {
		t.Fatal(err)
	}

	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutStr("llm.response.finish_reason", "stop")

	if err := p.ConsumeTraces(context.Background(), td); err != nil {
		t.Fatal(err)
	}

	out := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()
	v, ok := out.Get("gen_ai.response.finish_reasons")
	if !ok {
		t.Fatal("missing gen_ai.response.finish_reasons")
	}
	if v.Type() != pcommon.ValueTypeSlice {
		t.Fatalf("expected slice type, got %v", v.Type())
	}
	if v.Slice().Len() != 1 || v.Slice().At(0).Str() != "stop" {
		t.Errorf("expected [stop], got %v", v.Slice().AsRaw())
	}
}

func TestNormalizeSpanEvents(t *testing.T) {
	cfg := &Config{Profiles: []string{"openinference"}, RemoveOriginals: true}
	sink := new(consumertest.TracesSink)
	p, err := createTracesProcessor(context.Background(), processortest.NewNopSettings(component.MustNewType(typeStr)), cfg, sink)
	if err != nil {
		t.Fatal(err)
	}

	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutStr("llm.model_name", "claude-sonnet-4")

	evt := span.Events().AppendEmpty()
	evt.SetName("tool_call")
	evt.Attributes().PutStr("tool.name", "search")
	evt.Attributes().PutStr("tool_call.function.arguments", `{"q":"test"}`)

	if err := p.ConsumeTraces(context.Background(), td); err != nil {
		t.Fatal(err)
	}

	outSpan := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	assertAttrStr(t, outSpan.Attributes(), "gen_ai.request.model", "claude-sonnet-4")

	outEvt := outSpan.Events().At(0).Attributes()
	assertAttrStr(t, outEvt, "gen_ai.tool.name", "search")
	assertAttrStr(t, outEvt, "gen_ai.tool.call.arguments", `{"q":"test"}`)
	if _, ok := outEvt.Get("tool.name"); ok {
		t.Error("expected tool.name to be removed from event")
	}
}

func TestCustomMappings(t *testing.T) {
	cfg := &Config{
		Profiles:        []string{"openinference"},
		RemoveOriginals: true,
		CustomMappings: map[string]string{
			"my_vendor.model": "gen_ai.request.model",
		},
	}
	sink := new(consumertest.TracesSink)
	p, err := createTracesProcessor(context.Background(), processortest.NewNopSettings(component.MustNewType(typeStr)), cfg, sink)
	if err != nil {
		t.Fatal(err)
	}

	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutStr("my_vendor.model", "custom-model-v1")

	if err := p.ConsumeTraces(context.Background(), td); err != nil {
		t.Fatal(err)
	}

	out := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()
	assertAttrStr(t, out, "gen_ai.request.model", "custom-model-v1")
	if _, ok := out.Get("my_vendor.model"); ok {
		t.Error("expected my_vendor.model to be removed")
	}
}

func TestCustomMappingsOverrideProfile(t *testing.T) {
	cfg := &Config{
		Profiles:        []string{"openinference"},
		RemoveOriginals: true,
		CustomMappings: map[string]string{
			// Override the profile mapping for llm.model_name
			"llm.model_name": "gen_ai.response.model",
		},
	}
	sink := new(consumertest.TracesSink)
	p, err := createTracesProcessor(context.Background(), processortest.NewNopSettings(component.MustNewType(typeStr)), cfg, sink)
	if err != nil {
		t.Fatal(err)
	}

	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutStr("llm.model_name", "gpt-4o")

	if err := p.ConsumeTraces(context.Background(), td); err != nil {
		t.Fatal(err)
	}

	out := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()
	// Custom mapping should win over profile mapping
	assertAttrStr(t, out, "gen_ai.response.model", "gpt-4o")
	if _, ok := out.Get("gen_ai.request.model"); ok {
		t.Error("profile mapping should have been overridden by custom mapping")
	}
}

func TestOverwriteFalseSkipsExisting(t *testing.T) {
	cfg := &Config{Profiles: []string{"openinference"}, RemoveOriginals: true, Overwrite: false}
	sink := new(consumertest.TracesSink)
	p, err := createTracesProcessor(context.Background(), processortest.NewNopSettings(component.MustNewType(typeStr)), cfg, sink)
	if err != nil {
		t.Fatal(err)
	}

	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutStr("llm.model_name", "new-model")
	span.Attributes().PutStr("gen_ai.request.model", "existing-model")

	if err := p.ConsumeTraces(context.Background(), td); err != nil {
		t.Fatal(err)
	}

	out := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()
	assertAttrStr(t, out, "gen_ai.request.model", "existing-model")
}

func TestOverwriteTrueReplacesExisting(t *testing.T) {
	cfg := &Config{Profiles: []string{"openinference"}, RemoveOriginals: true, Overwrite: true}
	sink := new(consumertest.TracesSink)
	p, err := createTracesProcessor(context.Background(), processortest.NewNopSettings(component.MustNewType(typeStr)), cfg, sink)
	if err != nil {
		t.Fatal(err)
	}

	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutStr("llm.model_name", "new-model")
	span.Attributes().PutStr("gen_ai.request.model", "existing-model")

	if err := p.ConsumeTraces(context.Background(), td); err != nil {
		t.Fatal(err)
	}

	out := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()
	assertAttrStr(t, out, "gen_ai.request.model", "new-model")
}

func TestCreateProcessorRejectsInvalidConfig(t *testing.T) {
	cfg := &Config{Profiles: []string{"bogus"}}
	sink := new(consumertest.TracesSink)
	_, err := createTracesProcessor(context.Background(), processortest.NewNopSettings(component.MustNewType(typeStr)), cfg, sink)
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
}

func TestValueMappingThroughProcessor(t *testing.T) {
	cfg := &Config{Profiles: []string{"openinference"}, RemoveOriginals: true}
	sink := new(consumertest.TracesSink)
	p, err := createTracesProcessor(context.Background(), processortest.NewNopSettings(component.MustNewType(typeStr)), cfg, sink)
	if err != nil {
		t.Fatal(err)
	}

	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutStr("openinference.span.kind", "LLM")

	if err := p.ConsumeTraces(context.Background(), td); err != nil {
		t.Fatal(err)
	}

	out := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()
	assertAttrStr(t, out, "gen_ai.operation.name", "chat")
}

func assertAttrInt(t *testing.T, attrs pcommon.Map, key string, expected int64) {
	t.Helper()
	v, ok := attrs.Get(key)
	if !ok {
		t.Errorf("missing attribute %s", key)
		return
	}
	if v.Int() != expected {
		t.Errorf("%s = %d, want %d", key, v.Int(), expected)
	}
}

func assertAttrStr(t *testing.T, attrs pcommon.Map, key string, expected string) {
	t.Helper()
	v, ok := attrs.Get(key)
	if !ok {
		t.Errorf("missing attribute %s", key)
		return
	}
	if v.Str() != expected {
		t.Errorf("%s = %s, want %s", key, v.Str(), expected)
	}
}
