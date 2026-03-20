// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package genaiadapterconnector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func newTestHandler() *lloHandler {
	return newLLOHandler(zap.NewNop())
}

func newTestSpan(attrs map[string]string) ptrace.Span {
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	for k, v := range attrs {
		span.Attributes().PutStr(k, v)
	}
	return span
}

// test_llo_handler_patterns.py

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_patterns.py#L20
func TestIsLLOAttribute_GenAIIndexed(t *testing.T) {
	h := newTestHandler()
	assert.True(t, h.isLLOAttribute("gen_ai.prompt.0.content"))
	assert.True(t, h.isLLOAttribute("gen_ai.prompt.123.content"))
	assert.True(t, h.isLLOAttribute("gen_ai.completion.0.content"))
	assert.True(t, h.isLLOAttribute("gen_ai.completion.5.content"))
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_patterns.py#L27
func TestIsLLOAttribute_GenAIIndexed_NoMatch(t *testing.T) {
	h := newTestHandler()
	assert.False(t, h.isLLOAttribute("gen_ai.prompt.content"))
	assert.False(t, h.isLLOAttribute("gen_ai.prompt.abc.content"))
	assert.False(t, h.isLLOAttribute("some.other.attribute"))
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_patterns.py#L35
func TestIsLLOAttribute_Traceloop(t *testing.T) {
	h := newTestHandler()
	assert.True(t, h.isLLOAttribute("traceloop.entity.input"))
	assert.True(t, h.isLLOAttribute("traceloop.entity.output"))
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_patterns.py#L42
func TestIsLLOAttribute_OpenLit(t *testing.T) {
	h := newTestHandler()
	assert.True(t, h.isLLOAttribute("gen_ai.prompt"))
	assert.True(t, h.isLLOAttribute("gen_ai.completion"))
	assert.True(t, h.isLLOAttribute("gen_ai.content.revised_prompt"))
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_patterns.py#L50
func TestIsLLOAttribute_OpenInference(t *testing.T) {
	h := newTestHandler()
	assert.True(t, h.isLLOAttribute("input.value"))
	assert.True(t, h.isLLOAttribute("output.value"))
	assert.True(t, h.isLLOAttribute("llm.input_messages.0.message.content"))
	assert.True(t, h.isLLOAttribute("llm.output_messages.123.message.content"))
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_patterns.py#L60
func TestIsLLOAttribute_CrewAI(t *testing.T) {
	h := newTestHandler()
	assert.True(t, h.isLLOAttribute("crewai.crew.tasks_output"))
	assert.True(t, h.isLLOAttribute("crewai.crew.result"))
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_patterns.py#L69
func TestIsLLOAttribute_StrandsSDK(t *testing.T) {
	h := newTestHandler()
	assert.True(t, h.isLLOAttribute("system_prompt"))
	assert.True(t, h.isLLOAttribute("tool.result"))
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_patterns.py#L76
func TestIsLLOAttribute_LLMPrompts(t *testing.T) {
	h := newTestHandler()
	assert.True(t, h.isLLOAttribute("llm.prompts"))
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_patterns.py#L114
func TestIsLLOAttribute_OTelGenAI(t *testing.T) {
	h := newTestHandler()
	assert.True(t, h.isLLOAttribute("gen_ai.user.message"))
	assert.True(t, h.isLLOAttribute("gen_ai.assistant.message"))
	assert.True(t, h.isLLOAttribute("gen_ai.system.message"))
	assert.True(t, h.isLLOAttribute("gen_ai.tool.message"))
	assert.True(t, h.isLLOAttribute("gen_ai.choice"))
	assert.True(t, h.isLLOAttribute("gen_ai.input.messages"))
	assert.True(t, h.isLLOAttribute("gen_ai.output.messages"))
	assert.True(t, h.isLLOAttribute("gen_ai.system_instructions"))
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_patterns.py#L125
func TestIsLLOAttribute_OTelGenAI_NoMatch(t *testing.T) {
	h := newTestHandler()
	assert.False(t, h.isLLOAttribute("gen_ai.user"))
	assert.False(t, h.isLLOAttribute("gen_ai.assistant"))
	assert.False(t, h.isLLOAttribute("gen_ai.user.message.content"))
	assert.False(t, h.isLLOAttribute("gen_ai.invalid.message"))
}

func TestIsLLOAttribute_NonLLO(t *testing.T) {
	h := newTestHandler()
	assert.False(t, h.isLLOAttribute("http.method"))
	assert.False(t, h.isLLOAttribute("gen_ai.request.model"))
	assert.False(t, h.isLLOAttribute("gen_ai.usage.input_tokens"))
	assert.False(t, h.isLLOAttribute("gen_ai.operation.name"))
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_patterns.py#L13
func TestNewLLOHandler_NoPanic(t *testing.T) {
	h := newTestHandler()
	assert.NotNil(t, h)
	assert.NotEmpty(t, h.exactMatchPatterns)
	assert.NotEmpty(t, h.regexPatterns)
}

// test_llo_handler_processing.py

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_processing.py#L16
func TestRemoveLLOAttributes_RemovesLLO(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutStr("gen_ai.prompt.0.content", "Hello")
	span.Attributes().PutStr("gen_ai.prompt.0.role", "user")
	span.Attributes().PutStr("http.method", "POST")

	h.removeLLOAttributes(span)

	_, hasContent := span.Attributes().Get("gen_ai.prompt.0.content")
	assert.False(t, hasContent)
	_, hasRole := span.Attributes().Get("gen_ai.prompt.0.role")
	assert.True(t, hasRole)
	_, hasMethod := span.Attributes().Get("http.method")
	assert.True(t, hasMethod)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_processing.py#L35
func TestRemoveLLOAttributes_EmptyAttrs(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()

	h.removeLLOAttributes(span)

	assert.Equal(t, 0, span.Attributes().Len())
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_processing.py#L52
func TestRemoveLLOAttributes_NoLLO(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutStr("http.method", "POST")
	span.Attributes().PutInt("http.status_code", 200)

	h.removeLLOAttributes(span)

	assert.Equal(t, 2, span.Attributes().Len())
	v, ok := span.Attributes().Get("http.method")
	assert.True(t, ok)
	assert.Equal(t, "POST", v.AsString())
}

func TestRemoveLLOAttributes_AllFrameworks(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutStr("input.value", "user input")
	span.Attributes().PutStr("output.value", "assistant output")
	span.Attributes().PutStr("traceloop.entity.input", "traceloop input")
	span.Attributes().PutStr("traceloop.entity.output", "traceloop output")
	span.Attributes().PutStr("gen_ai.prompt", "openlit prompt")
	span.Attributes().PutStr("gen_ai.completion", "openlit completion")
	span.Attributes().PutStr("crewai.crew.tasks_output", "crew output")
	span.Attributes().PutStr("system_prompt", "system prompt")
	span.Attributes().PutStr("tool.result", "tool result")
	span.Attributes().PutStr("llm.prompts", "llm prompts")
	span.Attributes().PutStr("gen_ai.user.message", "user message")
	span.Attributes().PutStr("gen_ai.choice", "choice")
	span.Attributes().PutStr("gen_ai.input.messages", "[{\"role\":\"user\"}]")
	span.Attributes().PutStr("gen_ai.output.messages", "[{\"role\":\"assistant\"}]")
	span.Attributes().PutStr("http.method", "POST")
	span.Attributes().PutStr("gen_ai.request.model", "gpt-4")

	h.removeLLOAttributes(span)

	_, hasMethod := span.Attributes().Get("http.method")
	assert.True(t, hasMethod)
	_, hasModel := span.Attributes().Get("gen_ai.request.model")
	assert.True(t, hasModel)
	_, hasInput := span.Attributes().Get("input.value")
	assert.False(t, hasInput)
	_, hasOutput := span.Attributes().Get("output.value")
	assert.False(t, hasOutput)
	_, hasTraceloop := span.Attributes().Get("traceloop.entity.input")
	assert.False(t, hasTraceloop)
	_, hasPrompt := span.Attributes().Get("gen_ai.prompt")
	assert.False(t, hasPrompt)
	_, hasUserMsg := span.Attributes().Get("gen_ai.user.message")
	assert.False(t, hasUserMsg)
	_, hasInputMsgs := span.Attributes().Get("gen_ai.input.messages")
	assert.False(t, hasInputMsgs)
	_, hasOutputMsgs := span.Attributes().Get("gen_ai.output.messages")
	assert.False(t, hasOutputMsgs)
}

// test_llo_handler_events.py

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L385
func TestGroupMessagesByType_StandardRoles(t *testing.T) {
	h := newTestHandler()
	messages := []map[string]any{
		{"role": "system", "content": "system msg", "source": "prompt"},
		{"role": "user", "content": "user msg", "source": "prompt"},
		{"role": "assistant", "content": "assistant msg", "source": "completion"},
	}
	grouped := h.groupMessagesByType(messages)
	assert.Len(t, grouped["input"], 2)
	assert.Len(t, grouped["output"], 1)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L435
func TestGroupMessagesByType_ToolRouting(t *testing.T) {
	h := newTestHandler()
	messages := []map[string]any{
		{"role": "tool", "content": "tool from prompt", "source": "prompt"},
		{"role": "tool", "content": "tool from output", "source": "output"},
	}
	grouped := h.groupMessagesByType(messages)
	assert.Len(t, grouped["input"], 1)
	assert.Len(t, grouped["output"], 1)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L459
func TestGroupMessagesByType_EmptyList(t *testing.T) {
	h := newTestHandler()
	grouped := h.groupMessagesByType(nil)
	assert.Nil(t, grouped["input"])
	assert.Nil(t, grouped["output"])
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L469
func TestGroupMessagesByType_MissingFields(t *testing.T) {
	h := newTestHandler()
	messages := []map[string]any{
		{"content": "no role"},
		{"role": "user"},
	}
	grouped := h.groupMessagesByType(messages)
	assert.Len(t, grouped["input"], 2)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_processing.py#L237
func TestCollectLLOAttributesFromSpan(t *testing.T) {
	h := newTestHandler()
	span := newTestSpan(map[string]string{
		"input.value":          "hello",
		"output.value":         "world",
		"gen_ai.request.model": "gpt-4",
		"http.method":          "POST",
	})
	lloAttrs := h.collectLLOAttributesFromSpan(span)
	assert.Contains(t, lloAttrs, "input.value")
	assert.Contains(t, lloAttrs, "output.value")
	assert.NotContains(t, lloAttrs, "gen_ai.request.model")
	assert.NotContains(t, lloAttrs, "http.method")
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L883
func TestCollectLLOAttributesFromSpan_WithEvents(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutStr("http.method", "POST")

	event := span.Events().AppendEmpty()
	event.SetName("gen_ai.user.message")
	event.Attributes().PutStr("content", "Hello from event")

	lloAttrs := h.collectLLOAttributesFromSpan(span)
	assert.Contains(t, lloAttrs, "gen_ai.user.message")
}

// test_llo_handler_collection.py

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_collection.py#L144
func TestCollectAllLLOMessages_NilAttributes(t *testing.T) {
	h := newTestHandler()
	messages := h.collectAllLLOMessages(nil)
	assert.Empty(t, messages)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_frameworks.py#L168
func TestCollectAllLLOMessages_DirectPatterns(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"input.value":  "user input",
		"output.value": "assistant output",
	}
	messages := h.collectAllLLOMessages(attrs)
	assert.Len(t, messages, 2)
	for _, msg := range messages {
		assert.Contains(t, msg, "content")
		assert.Contains(t, msg, "role")
		assert.Contains(t, msg, "source")
	}
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_frameworks.py#L13
func TestCollectAllLLOMessages_Traceloop(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"traceloop.entity.input":  "traceloop input",
		"traceloop.entity.output": "traceloop output",
	}
	messages := h.collectAllLLOMessages(attrs)
	assert.Len(t, messages, 2)

	roles := map[string]bool{}
	for _, msg := range messages {
		roles[msg["role"].(string)] = true
	}
	assert.True(t, roles["user"])
	assert.True(t, roles["assistant"])
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_frameworks.py#L112
func TestCollectAllLLOMessages_OpenLit(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"gen_ai.prompt":                 "user prompt",
		"gen_ai.completion":             "assistant completion",
		"gen_ai.content.revised_prompt": "system revised",
	}
	messages := h.collectAllLLOMessages(attrs)
	assert.Len(t, messages, 3)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_frameworks.py#L314
func TestCollectAllLLOMessages_CrewAI(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"crewai.crew.tasks_output": "tasks output",
		"crewai.crew.result":       "crew result",
	}
	messages := h.collectAllLLOMessages(attrs)
	assert.Len(t, messages, 2)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_frameworks.py#L368
func TestCollectAllLLOMessages_StrandsSDK(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"system_prompt": "you are helpful",
		"tool.result":   "72F and sunny",
	}
	messages := h.collectAllLLOMessages(attrs)
	assert.Len(t, messages, 2)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_collection.py#L155
func TestCollectIndexedMessages_NilAttributes(t *testing.T) {
	h := newTestHandler()
	messages := h.collectIndexedMessages(nil)
	assert.Nil(t, messages)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_collection.py#L189
func TestCollectIndexedMessages_Sorted(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"gen_ai.prompt.2.content": "third",
		"gen_ai.prompt.2.role":    "user",
		"gen_ai.prompt.0.content": "first",
		"gen_ai.prompt.0.role":    "system",
		"gen_ai.prompt.1.content": "second",
		"gen_ai.prompt.1.role":    "user",
	}
	messages := h.collectIndexedMessages(attrs)
	require.Len(t, messages, 3)
	assert.Equal(t, "first", messages[0]["content"])
	assert.Equal(t, "second", messages[1]["content"])
	assert.Equal(t, "third", messages[2]["content"])
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_collection.py#L164
func TestCollectIndexedMessages_DefaultRole(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"gen_ai.prompt.0.content": "hello",
	}
	messages := h.collectIndexedMessages(attrs)
	require.Len(t, messages, 1)
	assert.Equal(t, "unknown", messages[0]["role"])
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_frameworks.py#L196
func TestCollectIndexedMessages_OpenInference(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"llm.input_messages.0.message.content":  "user message",
		"llm.input_messages.0.message.role":     "user",
		"llm.output_messages.0.message.content": "assistant message",
		"llm.output_messages.0.message.role":    "assistant",
	}
	messages := h.collectIndexedMessages(attrs)
	require.Len(t, messages, 2)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L944
func TestFilterSpanEvents_RemovesLLOPatternEvents(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()

	normalEvent := span.Events().AppendEmpty()
	normalEvent.SetName("normal.event")
	normalEvent.Attributes().PutStr("key", "value")

	lloEvent := span.Events().AppendEmpty()
	lloEvent.SetName("gen_ai.user.message")
	lloEvent.Attributes().PutStr("content", "hello")

	h.filterSpanEvents(span)

	assert.Equal(t, 1, span.Events().Len())
	assert.Equal(t, "normal.event", span.Events().At(0).Name())
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_processing.py#L169
func TestFilterSpanEvents_NoEvents(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	h.filterSpanEvents(span)
	assert.Equal(t, 0, span.Events().Len())
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_processing.py#L142
func TestFilterSpanEvents_RemovesLLOAttributesFromEvents(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()

	event := span.Events().AppendEmpty()
	event.SetName("some.event")
	event.Attributes().PutStr("gen_ai.prompt", "hello")
	event.Attributes().PutStr("normal.attr", "keep")

	h.filterSpanEvents(span)

	assert.Equal(t, 1, span.Events().Len())
	_, exists := span.Events().At(0).Attributes().Get("gen_ai.prompt")
	assert.False(t, exists)
	_, exists = span.Events().At(0).Attributes().Get("normal.attr")
	assert.True(t, exists)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L130
func TestEmitLLOAttributes_NoLLOAttrs(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()

	attrs := map[string]any{
		"http.method": "POST",
	}
	logs := h.emitLLOAttributes(span, rs.Resource(), pcommon.NewInstrumentationScope(), attrs, nil)
	assert.Equal(t, 0, logs.LogRecordCount())
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L210
func TestEmitLLOAttributes_NilAttrs(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()

	logs := h.emitLLOAttributes(span, rs.Resource(), pcommon.NewInstrumentationScope(), nil, nil)
	assert.Equal(t, 0, logs.LogRecordCount())
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L13
func TestEmitLLOAttributes_ProducesLogs(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("openinference.instrumentation.langchain")
	span := ss.Spans().AppendEmpty()

	attrs := map[string]any{
		"input.value":  "user input",
		"output.value": "assistant output",
	}
	logs := h.emitLLOAttributes(span, rs.Resource(), ss.Scope(), attrs, nil)
	assert.Equal(t, 1, logs.LogRecordCount())

	lr := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	eventName, _ := lr.Attributes().Get("event.name")
	assert.Equal(t, "openinference.instrumentation.langchain", eventName.AsString())

	body := lr.Body().Map()
	inputVal, hasInput := body.Get("input")
	outputVal, hasOutput := body.Get("output")
	assert.True(t, hasInput)
	assert.True(t, hasOutput)

	inputMessages := inputVal.Map()
	msgs, hasMsgs := inputMessages.Get("messages")
	assert.True(t, hasMsgs)
	assert.Equal(t, 1, msgs.Slice().Len())
	msg0 := msgs.Slice().At(0).Map()
	role, _ := msg0.Get("role")
	content, _ := msg0.Get("content")
	assert.Equal(t, "user", role.AsString())
	assert.Equal(t, "user input", content.AsString())

	outputMessages := outputVal.Map()
	oMsgs, hasOMsgs := outputMessages.Get("messages")
	assert.True(t, hasOMsgs)
	assert.Equal(t, 1, oMsgs.Slice().Len())
	oMsg0 := oMsgs.Slice().At(0).Map()
	oRole, _ := oMsg0.Get("role")
	oContent, _ := oMsg0.Get("content")
	assert.Equal(t, "assistant", oRole.AsString())
	assert.Equal(t, "assistant output", oContent.AsString())
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L591
func TestEmitLLOAttributes_SessionID(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.Attributes().PutStr("session.id", "sess-123")

	attrs := map[string]any{
		"input.value": "hello",
	}
	logs := h.emitLLOAttributes(span, rs.Resource(), ss.Scope(), attrs, nil)
	require.Equal(t, 1, logs.LogRecordCount())

	lr := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	sessionID, exists := lr.Attributes().Get("session.id")
	assert.True(t, exists)
	assert.Equal(t, "sess-123", sessionID.AsString())
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_processing.py#L68
func TestProcessSpans_FiltersAndEmits(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("test.scope")
	span := ss.Spans().AppendEmpty()
	span.Attributes().PutStr("input.value", "user input")
	span.Attributes().PutStr("output.value", "assistant output")
	span.Attributes().PutStr("http.method", "POST")

	logs := h.processSpans(td)

	assert.Equal(t, 1, logs.LogRecordCount())
	_, hasInput := span.Attributes().Get("input.value")
	assert.False(t, hasInput)
	_, hasOutput := span.Attributes().Get("output.value")
	assert.False(t, hasOutput)
	_, hasHTTP := span.Attributes().Get("http.method")
	assert.True(t, hasHTTP)
}

func TestProcessSpans_NoLLOAttributes(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutStr("http.method", "POST")

	logs := h.processSpans(td)
	assert.Equal(t, 0, logs.LogRecordCount())
	_, hasHTTP := span.Attributes().Get("http.method")
	assert.True(t, hasHTTP)
}

func TestProcessSpans_MultipleSpans(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("test.scope")

	span1 := ss.Spans().AppendEmpty()
	span1.Attributes().PutStr("input.value", "span1 input")

	span2 := ss.Spans().AppendEmpty()
	span2.Attributes().PutStr("gen_ai.prompt", "span2 prompt")
	span2.Attributes().PutStr("gen_ai.completion", "span2 completion")

	span3 := ss.Spans().AppendEmpty()
	span3.Attributes().PutStr("http.method", "GET")

	logs := h.processSpans(td)
	assert.Equal(t, 2, logs.LogRecordCount())
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_processing.py#L237
func TestProcessSpans_ConsolidatesSpanEvents(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("test.scope")
	span := ss.Spans().AppendEmpty()
	span.Attributes().PutStr("input.value", "from attrs")

	event := span.Events().AppendEmpty()
	event.SetName("gen_ai.user.message")
	event.Attributes().PutStr("content", "from event")

	logs := h.processSpans(td)
	assert.Equal(t, 1, logs.LogRecordCount())

	_, hasInput := span.Attributes().Get("input.value")
	assert.False(t, hasInput)
}

// test_llo_handler_collection.py

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_collection.py#L11
func TestCollectGenAIPromptMessages_SystemRole(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"gen_ai.prompt.0.content": "You are a helpful assistant",
		"gen_ai.prompt.0.role":    "system",
	}
	msgs := h.collectIndexedMessages(attrs)
	require.Len(t, msgs, 1)
	assert.Equal(t, "system", msgs[0]["role"])
	assert.Equal(t, "You are a helpful assistant", msgs[0]["content"])
	assert.Equal(t, "prompt", msgs[0]["source"])
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_collection.py#L30
func TestCollectGenAIPromptMessages_UserRole(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"gen_ai.prompt.0.content": "Hello",
		"gen_ai.prompt.0.role":    "user",
	}
	msgs := h.collectIndexedMessages(attrs)
	require.Len(t, msgs, 1)
	assert.Equal(t, "user", msgs[0]["role"])
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_collection.py#L49
func TestCollectGenAIPromptMessages_AssistantRole(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"gen_ai.prompt.1.content": "Hi there!",
		"gen_ai.prompt.1.role":    "assistant",
	}
	msgs := h.collectIndexedMessages(attrs)
	require.Len(t, msgs, 1)
	assert.Equal(t, "assistant", msgs[0]["role"])
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_collection.py#L68
func TestCollectGenAIPromptMessages_FunctionRole(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"gen_ai.prompt.2.content": "function result",
		"gen_ai.prompt.2.role":    "function",
	}
	msgs := h.collectIndexedMessages(attrs)
	require.Len(t, msgs, 1)
	assert.Equal(t, "function", msgs[0]["role"])
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_collection.py#L86
func TestCollectGenAIPromptMessages_UnknownRole(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"gen_ai.prompt.0.content": "hello",
		"gen_ai.prompt.0.role":    "custom_role",
	}
	msgs := h.collectIndexedMessages(attrs)
	require.Len(t, msgs, 1)
	assert.Equal(t, "custom_role", msgs[0]["role"])
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_collection.py#L104
func TestCollectGenAICompletionMessages_AssistantRole(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"gen_ai.completion.0.content": "I can help with that",
		"gen_ai.completion.0.role":    "assistant",
	}
	msgs := h.collectIndexedMessages(attrs)
	require.Len(t, msgs, 1)
	assert.Equal(t, "assistant", msgs[0]["role"])
	assert.Equal(t, "completion", msgs[0]["source"])
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_collection.py#L124
func TestCollectGenAICompletionMessages_OtherRole(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"gen_ai.completion.0.content": "result",
		"gen_ai.completion.0.role":    "other",
	}
	msgs := h.collectIndexedMessages(attrs)
	require.Len(t, msgs, 1)
	assert.Equal(t, "other", msgs[0]["role"])
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_collection.py#L222
func TestCollectMethodsMessageFormat(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"input.value":              "oi input",
		"output.value":             "oi output",
		"traceloop.entity.input":   "tl input",
		"gen_ai.prompt":            "openlit prompt",
		"crewai.crew.tasks_output": "crew output",
	}
	msgs := h.collectAllLLOMessages(attrs)
	for _, msg := range msgs {
		_, hasContent := msg["content"]
		_, hasRole := msg["role"]
		_, hasSource := msg["source"]
		assert.True(t, hasContent, "message missing content")
		assert.True(t, hasRole, "message missing role")
		assert.True(t, hasSource, "message missing source")
	}
}

// test_llo_handler_frameworks.py

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_frameworks.py#L13
func TestCollectTraceloopMessages_Detailed(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"traceloop.entity.input":  "user input",
		"traceloop.entity.output": "assistant output",
	}
	msgs := h.collectAllLLOMessages(attrs)
	require.Len(t, msgs, 2)
	roles := map[string]string{}
	for _, m := range msgs {
		roles[m["role"].(string)] = m["source"].(string)
	}
	assert.Equal(t, "input", roles["user"])
	assert.Equal(t, "output", roles["assistant"])
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_frameworks.py#L43
func TestCollectTraceloopMessages_AllAttributes(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"traceloop.entity.input":   "tl input",
		"traceloop.entity.output":  "tl output",
		"crewai.crew.tasks_output": "crew output",
		"crewai.crew.result":       "crew result",
	}
	msgs := h.collectAllLLOMessages(attrs)
	assert.Len(t, msgs, 4)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_frameworks.py#L79
func TestCollectOpenLitMessages_DirectPrompt(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{"gen_ai.prompt": "user prompt"}
	msgs := h.collectAllLLOMessages(attrs)
	require.Len(t, msgs, 1)
	assert.Equal(t, "user", msgs[0]["role"])
	assert.Equal(t, "prompt", msgs[0]["source"])
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_frameworks.py#L95
func TestCollectOpenLitMessages_DirectCompletion(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{"gen_ai.completion": "assistant completion"}
	msgs := h.collectAllLLOMessages(attrs)
	require.Len(t, msgs, 1)
	assert.Equal(t, "assistant", msgs[0]["role"])
	assert.Equal(t, "completion", msgs[0]["source"])
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_frameworks.py#L112
func TestCollectOpenLitMessages_AllAttributes(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"gen_ai.prompt":                 "prompt",
		"gen_ai.completion":             "completion",
		"gen_ai.content.revised_prompt": "revised",
		"gen_ai.agent.actual_output":    "agent output",
		"gen_ai.agent.human_input":      "agent input",
	}
	msgs := h.collectAllLLOMessages(attrs)
	assert.Len(t, msgs, 5)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_frameworks.py#L152
func TestCollectOpenLitMessages_RevisedPrompt(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{"gen_ai.content.revised_prompt": "revised system prompt"}
	msgs := h.collectAllLLOMessages(attrs)
	require.Len(t, msgs, 1)
	assert.Equal(t, "system", msgs[0]["role"])
	assert.Equal(t, "prompt", msgs[0]["source"])
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_frameworks.py#L196
func TestCollectOpenInferenceMessages_StructuredInput(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"llm.input_messages.0.message.content": "system msg",
		"llm.input_messages.0.message.role":    "system",
		"llm.input_messages.1.message.content": "user msg",
		"llm.input_messages.1.message.role":    "user",
	}
	msgs := h.collectIndexedMessages(attrs)
	require.Len(t, msgs, 2)
	assert.Equal(t, "system", msgs[0]["role"])
	assert.Equal(t, "user", msgs[1]["role"])
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_frameworks.py#L225
func TestCollectOpenInferenceMessages_StructuredOutput(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"llm.output_messages.0.message.content": "assistant response",
		"llm.output_messages.0.message.role":    "assistant",
	}
	msgs := h.collectIndexedMessages(attrs)
	require.Len(t, msgs, 1)
	assert.Equal(t, "assistant", msgs[0]["role"])
	assert.Equal(t, "output", msgs[0]["source"])
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_frameworks.py#L248
func TestCollectOpenInferenceMessages_MixedAttributes(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"input.value":                           "direct input",
		"output.value":                          "direct output",
		"llm.input_messages.0.message.content":  "structured input",
		"llm.input_messages.0.message.role":     "user",
		"llm.output_messages.0.message.content": "structured output",
		"llm.output_messages.0.message.role":    "assistant",
	}
	msgs := h.collectAllLLOMessages(attrs)
	assert.Len(t, msgs, 4)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_frameworks.py#L280
func TestCollectOpenLitMessages_AgentActualOutput(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{"gen_ai.agent.actual_output": "agent output"}
	msgs := h.collectAllLLOMessages(attrs)
	require.Len(t, msgs, 1)
	assert.Equal(t, "assistant", msgs[0]["role"])
	assert.Equal(t, "output", msgs[0]["source"])
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_frameworks.py#L298
func TestCollectOpenLitMessages_AgentHumanInput(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{"gen_ai.agent.human_input": "human input"}
	msgs := h.collectAllLLOMessages(attrs)
	require.Len(t, msgs, 1)
	assert.Equal(t, "user", msgs[0]["role"])
	assert.Equal(t, "input", msgs[0]["source"])
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_frameworks.py#L314
func TestCollectTraceloopMessages_CrewOutputs(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"crewai.crew.tasks_output": "tasks output",
		"crewai.crew.result":       "crew result",
	}
	msgs := h.collectAllLLOMessages(attrs)
	require.Len(t, msgs, 2)
	sources := map[string]bool{}
	for _, m := range msgs {
		sources[m["source"].(string)] = true
	}
	assert.True(t, sources["output"])
	assert.True(t, sources["result"])
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_frameworks.py#L342
func TestOpenInferenceMessages_WithDefaultRoles(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"llm.input_messages.0.message.content":  "input without role",
		"llm.output_messages.0.message.content": "output without role",
	}
	msgs := h.collectIndexedMessages(attrs)
	require.Len(t, msgs, 2)
	for _, m := range msgs {
		source := m["source"].(string)
		if source == "input" {
			assert.Equal(t, "user", m["role"])
		} else {
			assert.Equal(t, "assistant", m["role"])
		}
	}
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_frameworks.py#L368
func TestCollectStrandsSDKMessages_Detailed(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"system_prompt": "you are helpful",
		"tool.result":   "72F and sunny",
	}
	msgs := h.collectAllLLOMessages(attrs)
	require.Len(t, msgs, 2)
	roles := map[string]string{}
	for _, m := range msgs {
		roles[m["role"].(string)] = m["content"].(string)
	}
	assert.Equal(t, "you are helpful", roles["system"])
	assert.Equal(t, "72F and sunny", roles["assistant"])
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_frameworks.py#L397
func TestCollectLLMPromptsMessages(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"llm.prompts": `[{"role": "user", "content": "Hello"}]`,
	}
	msgs := h.collectAllLLOMessages(attrs)
	require.Len(t, msgs, 1)
	assert.Equal(t, "user", msgs[0]["role"])
	assert.Equal(t, "prompt", msgs[0]["source"])
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_frameworks.py#L418
func TestCollectLLMPrompts_WithOtherMessages(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"llm.prompts":       `[{"role": "user", "content": "Hello"}]`,
		"gen_ai.prompt":     "openlit prompt",
		"gen_ai.completion": "openlit completion",
	}
	msgs := h.collectAllLLOMessages(attrs)
	assert.Len(t, msgs, 3)
}

// test_llo_handler_events.py

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L13
func TestEmitLLOAttributes_Full(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("test.scope")
	span := ss.Spans().AppendEmpty()

	attrs := map[string]any{
		"gen_ai.prompt.0.content":     "Hello",
		"gen_ai.prompt.0.role":        "user",
		"gen_ai.completion.0.content": "Hi there",
		"gen_ai.completion.0.role":    "assistant",
	}
	logs := h.emitLLOAttributes(span, rs.Resource(), ss.Scope(), attrs, nil)
	require.Equal(t, 1, logs.LogRecordCount())

	lr := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	body := lr.Body().Map()
	_, hasInput := body.Get("input")
	_, hasOutput := body.Get("output")
	assert.True(t, hasInput)
	assert.True(t, hasOutput)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L74
func TestEmitLLOAttributes_MultipleFrameworks(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()

	attrs := map[string]any{
		"input.value":             "oi input",
		"output.value":            "oi output",
		"traceloop.entity.input":  "tl input",
		"traceloop.entity.output": "tl output",
		"gen_ai.prompt":           "openlit prompt",
	}
	logs := h.emitLLOAttributes(span, rs.Resource(), ss.Scope(), attrs, nil)
	assert.Equal(t, 1, logs.LogRecordCount())
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L147
func TestEmitLLOAttributes_MixedInputOutput(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()

	attrs := map[string]any{
		"gen_ai.prompt.0.content":     "system msg",
		"gen_ai.prompt.0.role":        "system",
		"gen_ai.prompt.1.content":     "user msg",
		"gen_ai.prompt.1.role":        "user",
		"gen_ai.completion.0.content": "assistant msg",
		"gen_ai.completion.0.role":    "assistant",
	}
	logs := h.emitLLOAttributes(span, rs.Resource(), ss.Scope(), attrs, nil)
	require.Equal(t, 1, logs.LogRecordCount())

	body := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map()
	_, hasInput := body.Get("input")
	_, hasOutput := body.Get("output")
	assert.True(t, hasInput)
	assert.True(t, hasOutput)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L189
func TestEmitLLOAttributes_WithEventTimestamp(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()

	attrs := map[string]any{"input.value": "hello"}
	ts := pcommon.NewTimestampFromTime(span.EndTimestamp().AsTime().Add(1000))
	logs := h.emitLLOAttributes(span, rs.Resource(), ss.Scope(), attrs, &ts)
	require.Equal(t, 1, logs.LogRecordCount())

	lr := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	assert.Equal(t, ts, lr.Timestamp())
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L302
func TestEmitLLOAttributes_OnlyInputMessages(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()

	attrs := map[string]any{"input.value": "user input only"}
	logs := h.emitLLOAttributes(span, rs.Resource(), ss.Scope(), attrs, nil)
	require.Equal(t, 1, logs.LogRecordCount())

	body := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map()
	_, hasInput := body.Get("input")
	_, hasOutput := body.Get("output")
	assert.True(t, hasInput)
	assert.False(t, hasOutput)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L331
func TestEmitLLOAttributes_OnlyOutputMessages(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()

	attrs := map[string]any{"output.value": "assistant output only"}
	logs := h.emitLLOAttributes(span, rs.Resource(), ss.Scope(), attrs, nil)
	require.Equal(t, 1, logs.LogRecordCount())

	body := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map()
	_, hasInput := body.Get("input")
	_, hasOutput := body.Get("output")
	assert.False(t, hasInput)
	assert.True(t, hasOutput)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L622
func TestEmitLLOAttributes_WithoutSessionID(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()

	attrs := map[string]any{"input.value": "hello"}
	logs := h.emitLLOAttributes(span, rs.Resource(), ss.Scope(), attrs, nil)
	require.Equal(t, 1, logs.LogRecordCount())

	lr := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	_, exists := lr.Attributes().Get("session.id")
	assert.False(t, exists)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L647
func TestEmitLLOAttributes_SessionIDAndOtherAttributes(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.Attributes().PutStr("session.id", "sess-456")
	span.Attributes().PutStr("user.id", "user-789")
	span.Attributes().PutStr("other.attribute", "should not copy")

	attrs := map[string]any{"input.value": "hello"}
	logs := h.emitLLOAttributes(span, rs.Resource(), ss.Scope(), attrs, nil)
	require.Equal(t, 1, logs.LogRecordCount())

	lr := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	sessionID, exists := lr.Attributes().Get("session.id")
	assert.True(t, exists)
	assert.Equal(t, "sess-456", sessionID.AsString())
	_, hasUserID := lr.Attributes().Get("user.id")
	assert.False(t, hasUserID)
	_, hasOther := lr.Attributes().Get("other.attribute")
	assert.False(t, hasOther)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L679
func TestEmitLLOAttributes_OTelGenAIPatterns(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()

	attrs := map[string]any{
		"gen_ai.user.message":      "user msg",
		"gen_ai.assistant.message": "assistant msg",
		"gen_ai.system.message":    "system msg",
		"gen_ai.tool.message":      "tool msg",
		"gen_ai.choice":            "choice msg",
	}
	logs := h.emitLLOAttributes(span, rs.Resource(), ss.Scope(), attrs, nil)
	require.Equal(t, 1, logs.LogRecordCount())

	body := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map()
	_, hasInput := body.Get("input")
	_, hasOutput := body.Get("output")
	assert.True(t, hasInput)
	assert.True(t, hasOutput)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L746
func TestEmitLLOAttributes_GenAIUserMessageOnly(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()

	attrs := map[string]any{"gen_ai.user.message": "hello"}
	logs := h.emitLLOAttributes(span, rs.Resource(), ss.Scope(), attrs, nil)
	require.Equal(t, 1, logs.LogRecordCount())

	body := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map()
	_, hasInput := body.Get("input")
	_, hasOutput := body.Get("output")
	assert.True(t, hasInput)
	assert.False(t, hasOutput)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L773
func TestEmitLLOAttributes_GenAIAssistantMessageOnly(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()

	attrs := map[string]any{"gen_ai.assistant.message": "response"}
	logs := h.emitLLOAttributes(span, rs.Resource(), ss.Scope(), attrs, nil)
	require.Equal(t, 1, logs.LogRecordCount())

	body := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map()
	_, hasInput := body.Get("input")
	_, hasOutput := body.Get("output")
	assert.False(t, hasInput)
	assert.True(t, hasOutput)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L800
func TestEmitLLOAttributes_GenAISystemMessageOnly(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()

	attrs := map[string]any{"gen_ai.system.message": "system instructions"}
	logs := h.emitLLOAttributes(span, rs.Resource(), ss.Scope(), attrs, nil)
	require.Equal(t, 1, logs.LogRecordCount())

	body := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map()
	inputVal, hasInput := body.Get("input")
	_, hasOutput := body.Get("output")
	assert.True(t, hasInput)
	assert.False(t, hasOutput)

	inputMessages := inputVal.Map()
	msgs, hasMsgs := inputMessages.Get("messages")
	assert.True(t, hasMsgs)
	assert.Equal(t, 1, msgs.Slice().Len())
	msg0 := msgs.Slice().At(0).Map()
	role, _ := msg0.Get("role")
	content, _ := msg0.Get("content")
	assert.Equal(t, "system", role.AsString())
	assert.Equal(t, "system instructions", content.AsString())
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L829
func TestEmitLLOAttributes_GenAIToolMessageOnly(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()

	attrs := map[string]any{"gen_ai.tool.message": "tool result"}
	logs := h.emitLLOAttributes(span, rs.Resource(), ss.Scope(), attrs, nil)
	require.Equal(t, 1, logs.LogRecordCount())

	body := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map()
	_, hasInput := body.Get("input")
	assert.True(t, hasInput)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L856
func TestEmitLLOAttributes_GenAIChoiceOnly(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()

	attrs := map[string]any{"gen_ai.choice": "chosen response"}
	logs := h.emitLLOAttributes(span, rs.Resource(), ss.Scope(), attrs, nil)
	require.Equal(t, 1, logs.LogRecordCount())

	body := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map()
	_, hasOutput := body.Get("output")
	assert.True(t, hasOutput)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L883
func TestEmitLLOAttributes_FromSpanEvents(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()

	event := span.Events().AppendEmpty()
	event.SetName("gen_ai.user.message")
	event.Attributes().PutStr("content", "event content")

	lloAttrs := h.collectLLOAttributesFromSpan(span)
	assert.Contains(t, lloAttrs, "gen_ai.user.message")
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L490
func TestEmitLLOAttributes_WithLLMPrompts(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()

	attrs := map[string]any{
		"llm.prompts":                 `[{"role": "user", "content": "Hello"}]`,
		"gen_ai.completion.0.content": "response",
		"gen_ai.completion.0.role":    "assistant",
	}
	logs := h.emitLLOAttributes(span, rs.Resource(), ss.Scope(), attrs, nil)
	require.Equal(t, 1, logs.LogRecordCount())

	body := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map()
	_, hasInput := body.Get("input")
	_, hasOutput := body.Get("output")
	assert.True(t, hasInput)
	assert.True(t, hasOutput)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_events.py#L409
func TestGroupMessagesByType_NonStandardRoles(t *testing.T) {
	h := newTestHandler()
	messages := []map[string]any{
		{"role": "function", "content": "func from prompt", "source": "prompt"},
		{"role": "custom", "content": "custom from completion", "source": "completion"},
	}
	grouped := h.groupMessagesByType(messages)
	assert.Len(t, grouped["input"], 1)
	assert.Len(t, grouped["output"], 1)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/tests/amazon/opentelemetry/distro/llo_handler/test_llo_handler_processing.py#L181
func TestFilterSpanEvents_NoAttributes(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()

	event := span.Events().AppendEmpty()
	event.SetName("normal.event")

	h.filterSpanEvents(span)
	assert.Equal(t, 1, span.Events().Len())
}

func TestIsLLOAttribute_SystemInstructions(t *testing.T) {
	h := newTestHandler()
	assert.True(t, h.isLLOAttribute("gen_ai.system_instructions"))
}

func TestCollectAllLLOMessages_SystemInstructions(t *testing.T) {
	h := newTestHandler()
	attrs := map[string]any{
		"gen_ai.system_instructions": "You are a helpful assistant",
	}
	msgs := h.collectAllLLOMessages(attrs)
	require.Len(t, msgs, 1)
	assert.Equal(t, "system", msgs[0]["role"])
	assert.Equal(t, "prompt", msgs[0]["source"])
	assert.Equal(t, "You are a helpful assistant", msgs[0]["content"])
}

func TestEmitLLOAttributes_SystemInstructionsOnly(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()

	attrs := map[string]any{"gen_ai.system_instructions": "Be concise"}
	logs := h.emitLLOAttributes(span, rs.Resource(), ss.Scope(), attrs, nil)
	require.Equal(t, 1, logs.LogRecordCount())

	body := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map()
	inputVal, hasInput := body.Get("input")
	_, hasOutput := body.Get("output")
	assert.True(t, hasInput)
	assert.False(t, hasOutput)

	inputMessages := inputVal.Map()
	msgs, hasMsgs := inputMessages.Get("messages")
	assert.True(t, hasMsgs)
	assert.Equal(t, 1, msgs.Slice().Len())
	msg0 := msgs.Slice().At(0).Map()
	role, _ := msg0.Get("role")
	content, _ := msg0.Get("content")
	assert.Equal(t, "system", role.AsString())
	assert.Equal(t, "Be concise", content.AsString())
}

func TestEmitLLOAttributes_SystemInstructionsWithMessages(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()

	attrs := map[string]any{
		"gen_ai.system_instructions": "You are helpful",
		"gen_ai.input.messages":      "[{\"role\":\"user\",\"content\":\"Hello\"}]",
		"gen_ai.output.messages":     "[{\"role\":\"assistant\",\"content\":\"Hi\"}]",
	}
	logs := h.emitLLOAttributes(span, rs.Resource(), ss.Scope(), attrs, nil)
	require.Equal(t, 1, logs.LogRecordCount())

	body := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map()
	inputVal, hasInput := body.Get("input")
	outputVal, hasOutput := body.Get("output")
	assert.True(t, hasInput)
	assert.True(t, hasOutput)

	inputMessages := inputVal.Map()
	msgs, _ := inputMessages.Get("messages")
	found := false
	for i := 0; i < msgs.Slice().Len(); i++ {
		m := msgs.Slice().At(i).Map()
		r, _ := m.Get("role")
		c, _ := m.Get("content")
		if r.AsString() == "system" && c.AsString() == "You are helpful" {
			found = true
		}
	}
	assert.True(t, found)

	outputMessages := outputVal.Map()
	oMsgs, _ := outputMessages.Get("messages")
	assert.GreaterOrEqual(t, oMsgs.Slice().Len(), 1)
}

func TestRemoveLLOAttributes_PreservesNonLLO(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutStr("keep", "preserved")
	span.Attributes().PutInt("keep_int", 99)
	span.Attributes().PutDouble("keep_float", 3.14)
	span.Attributes().PutBool("keep_bool", true)
	span.Attributes().PutStr("input.value", "remove me")

	h.removeLLOAttributes(span)

	assert.Equal(t, 4, span.Attributes().Len())

	v, ok := span.Attributes().Get("keep")
	assert.True(t, ok)
	assert.Equal(t, "preserved", v.AsString())

	iv, ok := span.Attributes().Get("keep_int")
	assert.True(t, ok)
	assert.Equal(t, int64(99), iv.Int())

	fv, ok := span.Attributes().Get("keep_float")
	assert.True(t, ok)
	assert.InDelta(t, 3.14, fv.Double(), 0.001)

	bv, ok := span.Attributes().Get("keep_bool")
	assert.True(t, ok)
	assert.True(t, bv.Bool())

	_, hasLLO := span.Attributes().Get("input.value")
	assert.False(t, hasLLO)
}

func TestRemoveLLOAttributes_PreservesOriginalTypes(t *testing.T) {
	h := newTestHandler()
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutStr("http.method", "POST")
	span.Attributes().PutInt("http.status_code", 200)
	span.Attributes().PutStr("gen_ai.prompt", "remove me")

	h.removeLLOAttributes(span)

	assert.Equal(t, 2, span.Attributes().Len())

	method, ok := span.Attributes().Get("http.method")
	assert.True(t, ok)
	assert.Equal(t, "POST", method.AsString())

	status, ok := span.Attributes().Get("http.status_code")
	assert.True(t, ok)
	assert.Equal(t, int64(200), status.Int())
}
