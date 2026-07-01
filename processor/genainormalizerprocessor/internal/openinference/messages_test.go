// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openinference

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"
)

func newAttrs(kvs map[string]string) pcommon.Map {
	m := pcommon.NewMap()
	for k, v := range kvs {
		m.PutStr(k, v)
	}
	return m
}

func parseJSON(t *testing.T, s string) []any {
	t.Helper()
	var out []any
	require.NoError(t, json.Unmarshal([]byte(s), &out))
	return out
}

func TestReconstructMessages_BasicInputMessages(t *testing.T) {
	attrs := newAttrs(map[string]string{
		"llm.input_messages.0.message.role":    "system",
		"llm.input_messages.0.message.content": "You are helpful.",
		"llm.input_messages.1.message.role":    "user",
		"llm.input_messages.1.message.content": "Hello",
	})

	wrote := ReconstructMessages(attrs, true, false)
	require.True(t, wrote)

	val, ok := attrs.Get(otelsemconv.GenAIInputMessages)
	require.True(t, ok)

	msgs := parseJSON(t, val.AsString())
	require.Len(t, msgs, 2)

	msg0 := msgs[0].(map[string]any)
	assert.Equal(t, "system", msg0["role"])
	parts0 := msg0["parts"].([]any)
	require.Len(t, parts0, 1)
	part0 := parts0[0].(map[string]any)
	assert.Equal(t, "text", part0["type"])
	assert.Equal(t, "You are helpful.", part0["content"])

	msg1 := msgs[1].(map[string]any)
	assert.Equal(t, "user", msg1["role"])

	_, exists := attrs.Get("llm.input_messages.0.message.role")
	assert.False(t, exists, "originals should be removed")
}

func TestReconstructMessages_OutputMessages(t *testing.T) {
	attrs := newAttrs(map[string]string{
		"llm.output_messages.0.message.role":    "assistant",
		"llm.output_messages.0.message.content": "Hi there!",
	})

	wrote := ReconstructMessages(attrs, true, false)
	require.True(t, wrote)

	val, ok := attrs.Get(otelsemconv.GenAIOutputMessages)
	require.True(t, ok)

	msgs := parseJSON(t, val.AsString())
	require.Len(t, msgs, 1)
	msg := msgs[0].(map[string]any)
	assert.Equal(t, "assistant", msg["role"])
	// finish_reason is required by the GenAI output-messages schema; emitted as ""
	// because OpenInference does not carry per-message finish reasons.
	assert.Empty(t, msg["finish_reason"])
}

func TestReconstructMessages_ToolCalls(t *testing.T) {
	attrs := newAttrs(map[string]string{
		"llm.output_messages.0.message.role":                                      "assistant",
		"llm.output_messages.0.message.tool_calls.0.tool_call.id":                 "call_abc",
		"llm.output_messages.0.message.tool_calls.0.tool_call.function.name":      "get_weather",
		"llm.output_messages.0.message.tool_calls.0.tool_call.function.arguments": `{"city":"Berlin"}`,
	})

	wrote := ReconstructMessages(attrs, true, false)
	require.True(t, wrote)

	val, ok := attrs.Get(otelsemconv.GenAIOutputMessages)
	require.True(t, ok)

	msgs := parseJSON(t, val.AsString())
	require.Len(t, msgs, 1)

	msg := msgs[0].(map[string]any)
	assert.Equal(t, "assistant", msg["role"])
	parts := msg["parts"].([]any)
	require.Len(t, parts, 1)

	tc := parts[0].(map[string]any)
	assert.Equal(t, "tool_call", tc["type"])
	assert.Equal(t, "call_abc", tc["id"])
	assert.Equal(t, "get_weather", tc["name"])
	args := tc["arguments"].(map[string]any)
	assert.Equal(t, "Berlin", args["city"])
}

func TestReconstructMessages_ToolResponse(t *testing.T) {
	attrs := newAttrs(map[string]string{
		"llm.input_messages.2.message.role":         "user",
		"llm.input_messages.2.message.content":      "sunny, 22C",
		"llm.input_messages.2.message.tool_call_id": "call_abc",
	})

	wrote := ReconstructMessages(attrs, true, false)
	require.True(t, wrote)

	val, ok := attrs.Get(otelsemconv.GenAIInputMessages)
	require.True(t, ok)

	msgs := parseJSON(t, val.AsString())
	require.Len(t, msgs, 1)

	msg := msgs[0].(map[string]any)
	assert.Equal(t, "tool", msg["role"], "role should be inferred as tool when tool_call_id present")

	parts := msg["parts"].([]any)
	require.Len(t, parts, 1)
	part := parts[0].(map[string]any)
	assert.Equal(t, "tool_call_response", part["type"])
	assert.Equal(t, "call_abc", part["id"])
	assert.Equal(t, "sunny, 22C", part["response"])
}

func TestReconstructMessages_ToolResponseExplicitRole(t *testing.T) {
	attrs := newAttrs(map[string]string{
		"llm.input_messages.0.message.role":         "tool",
		"llm.input_messages.0.message.content":      "result data",
		"llm.input_messages.0.message.tool_call_id": "call_xyz",
	})

	wrote := ReconstructMessages(attrs, true, false)
	require.True(t, wrote)

	val, _ := attrs.Get(otelsemconv.GenAIInputMessages)
	msgs := parseJSON(t, val.AsString())
	assert.Equal(t, "tool", msgs[0].(map[string]any)["role"])
}

func TestReconstructMessages_InferredRoles(t *testing.T) {
	tests := []struct {
		name         string
		attrs        map[string]string
		expectedRole string
	}{
		{
			name: "plain content defaults to user",
			attrs: map[string]string{
				"llm.input_messages.0.message.content": "hello",
			},
			expectedRole: "user",
		},
		{
			name: "tool_calls without role defaults to assistant",
			attrs: map[string]string{
				"llm.output_messages.0.message.tool_calls.0.tool_call.function.name": "fn",
				"llm.output_messages.0.message.tool_calls.0.tool_call.id":            "c1",
			},
			expectedRole: "assistant",
		},
		{
			name: "tool_call_id without role defaults to tool",
			attrs: map[string]string{
				"llm.input_messages.0.message.content":      "res",
				"llm.input_messages.0.message.tool_call_id": "c1",
			},
			expectedRole: "tool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newAttrs(tt.attrs)
			wrote := ReconstructMessages(a, true, false)
			require.True(t, wrote)

			var target string
			if _, ok := a.Get(otelsemconv.GenAIInputMessages); ok {
				target = otelsemconv.GenAIInputMessages
			} else {
				target = otelsemconv.GenAIOutputMessages
			}
			val, _ := a.Get(target)
			msgs := parseJSON(t, val.AsString())
			require.Len(t, msgs, 1)
			assert.Equal(t, tt.expectedRole, msgs[0].(map[string]any)["role"])
		})
	}
}

func TestReconstructMessages_MessageName(t *testing.T) {
	attrs := newAttrs(map[string]string{
		"llm.input_messages.0.message.role":    "assistant",
		"llm.input_messages.0.message.name":    "helper_bot",
		"llm.input_messages.0.message.content": "Hi",
	})

	wrote := ReconstructMessages(attrs, true, false)
	require.True(t, wrote)

	val, _ := attrs.Get(otelsemconv.GenAIInputMessages)
	msgs := parseJSON(t, val.AsString())
	require.Len(t, msgs, 1)
	msg := msgs[0].(map[string]any)
	assert.Equal(t, "assistant", msg["role"])
	assert.Equal(t, "helper_bot", msg["name"], "name field must appear in the JSON output")

	_, exists := attrs.Get("llm.input_messages.0.message.name")
	assert.False(t, exists, "name attr should be consumed")
}

func TestReconstructMessages_MessageNameOutput(t *testing.T) {
	attrs := newAttrs(map[string]string{
		"llm.output_messages.0.message.role":    "assistant",
		"llm.output_messages.0.message.name":    "my_agent",
		"llm.output_messages.0.message.content": "Done",
	})

	wrote := ReconstructMessages(attrs, true, false)
	require.True(t, wrote)

	val, _ := attrs.Get(otelsemconv.GenAIOutputMessages)
	msgs := parseJSON(t, val.AsString())
	require.Len(t, msgs, 1)
	msg := msgs[0].(map[string]any)
	assert.Equal(t, "my_agent", msg["name"])
	assert.Empty(t, msg["finish_reason"])
}

func TestReconstructMessages_MessageNameOmittedWhenEmpty(t *testing.T) {
	attrs := newAttrs(map[string]string{
		"llm.input_messages.0.message.role":    "user",
		"llm.input_messages.0.message.content": "Hi",
	})

	wrote := ReconstructMessages(attrs, true, false)
	require.True(t, wrote)

	val, _ := attrs.Get(otelsemconv.GenAIInputMessages)
	msgs := parseJSON(t, val.AsString())
	require.Len(t, msgs, 1)
	_, hasName := msgs[0].(map[string]any)["name"]
	assert.False(t, hasName, "name must be omitted when not present in source")
}

func TestReconstructMessages_NoOverwrite(t *testing.T) {
	attrs := newAttrs(map[string]string{
		"llm.input_messages.0.message.role":    "user",
		"llm.input_messages.0.message.content": "hello",
	})
	attrs.PutStr(otelsemconv.GenAIInputMessages, "existing")

	wrote := ReconstructMessages(attrs, true, false)
	assert.False(t, wrote)

	val, _ := attrs.Get(otelsemconv.GenAIInputMessages)
	assert.Equal(t, "existing", val.AsString())
}

func TestReconstructMessages_Overwrite(t *testing.T) {
	attrs := newAttrs(map[string]string{
		"llm.input_messages.0.message.role":    "user",
		"llm.input_messages.0.message.content": "hello",
	})
	attrs.PutStr(otelsemconv.GenAIInputMessages, "existing")

	wrote := ReconstructMessages(attrs, true, true)
	assert.True(t, wrote)

	val, _ := attrs.Get(otelsemconv.GenAIInputMessages)
	assert.NotEqual(t, "existing", val.AsString())
}

func TestReconstructMessages_KeepOriginals(t *testing.T) {
	attrs := newAttrs(map[string]string{
		"llm.input_messages.0.message.role":    "user",
		"llm.input_messages.0.message.content": "hello",
	})

	wrote := ReconstructMessages(attrs, false, false)
	require.True(t, wrote)

	_, ok := attrs.Get("llm.input_messages.0.message.role")
	assert.True(t, ok, "originals should be kept when removeOriginals=false")
}

func TestReconstructMessages_NoFlattenedAttrs(t *testing.T) {
	attrs := newAttrs(map[string]string{
		"llm.model_name": "gpt-4",
	})

	wrote := ReconstructMessages(attrs, true, false)
	assert.False(t, wrote)
}

func TestReconstructMessages_InvalidIndex(t *testing.T) {
	attrs := newAttrs(map[string]string{
		"llm.input_messages.abc.message.role": "user",
	})

	wrote := ReconstructMessages(attrs, true, false)
	assert.False(t, wrote)
}

func TestReconstructMessages_NonStringValue(t *testing.T) {
	m := pcommon.NewMap()
	m.PutStr("llm.input_messages.0.message.role", "user")
	m.PutInt("llm.input_messages.0.message.content", 42)

	wrote := ReconstructMessages(m, true, false)
	require.True(t, wrote)

	val, _ := m.Get(otelsemconv.GenAIInputMessages)
	msgs := parseJSON(t, val.AsString())
	require.Len(t, msgs, 1)
	parts := msgs[0].(map[string]any)["parts"].([]any)
	require.Len(t, parts, 1)
	assert.Equal(t, "42", parts[0].(map[string]any)["content"])
}

func TestReconstructMessages_MultipleToolCalls(t *testing.T) {
	attrs := newAttrs(map[string]string{
		"llm.output_messages.0.message.role":                                      "assistant",
		"llm.output_messages.0.message.tool_calls.0.tool_call.id":                 "c1",
		"llm.output_messages.0.message.tool_calls.0.tool_call.function.name":      "fn1",
		"llm.output_messages.0.message.tool_calls.0.tool_call.function.arguments": `{"a":1}`,
		"llm.output_messages.0.message.tool_calls.1.tool_call.id":                 "c2",
		"llm.output_messages.0.message.tool_calls.1.tool_call.function.name":      "fn2",
		"llm.output_messages.0.message.tool_calls.1.tool_call.function.arguments": `invalid json`,
	})

	wrote := ReconstructMessages(attrs, true, false)
	require.True(t, wrote)

	val, _ := attrs.Get(otelsemconv.GenAIOutputMessages)
	msgs := parseJSON(t, val.AsString())
	require.Len(t, msgs, 1)

	parts := msgs[0].(map[string]any)["parts"].([]any)
	require.Len(t, parts, 2)

	tc0 := parts[0].(map[string]any)
	assert.Equal(t, "fn1", tc0["name"])
	assert.Equal(t, float64(1), tc0["arguments"].(map[string]any)["a"])

	tc1 := parts[1].(map[string]any)
	assert.Equal(t, "fn2", tc1["name"])
	assert.Equal(t, "invalid json", tc1["arguments"])
}

func TestReconstructMessages_OrderingByIndex(t *testing.T) {
	attrs := newAttrs(map[string]string{
		"llm.input_messages.2.message.role":    "assistant",
		"llm.input_messages.2.message.content": "third",
		"llm.input_messages.0.message.role":    "system",
		"llm.input_messages.0.message.content": "first",
		"llm.input_messages.1.message.role":    "user",
		"llm.input_messages.1.message.content": "second",
	})

	wrote := ReconstructMessages(attrs, true, false)
	require.True(t, wrote)

	val, _ := attrs.Get(otelsemconv.GenAIInputMessages)
	msgs := parseJSON(t, val.AsString())
	require.Len(t, msgs, 3)
	assert.Equal(t, "system", msgs[0].(map[string]any)["role"])
	assert.Equal(t, "user", msgs[1].(map[string]any)["role"])
	assert.Equal(t, "assistant", msgs[2].(map[string]any)["role"])
}

func TestReconstructMessages_EmptyContent(t *testing.T) {
	attrs := newAttrs(map[string]string{
		"llm.input_messages.0.message.role": "user",
	})

	wrote := ReconstructMessages(attrs, true, false)
	require.True(t, wrote)

	val, _ := attrs.Get(otelsemconv.GenAIInputMessages)
	msgs := parseJSON(t, val.AsString())
	require.Len(t, msgs, 1)
	parts := msgs[0].(map[string]any)["parts"].([]any)
	assert.Empty(t, parts)
}

func TestReconstructMessages_BothInputAndOutput(t *testing.T) {
	attrs := newAttrs(map[string]string{
		"llm.input_messages.0.message.role":     "user",
		"llm.input_messages.0.message.content":  "question",
		"llm.output_messages.0.message.role":    "assistant",
		"llm.output_messages.0.message.content": "answer",
	})

	wrote := ReconstructMessages(attrs, true, false)
	require.True(t, wrote)

	inVal, ok := attrs.Get(otelsemconv.GenAIInputMessages)
	require.True(t, ok)
	outVal, ok := attrs.Get(otelsemconv.GenAIOutputMessages)
	require.True(t, ok)

	inMsgs := parseJSON(t, inVal.AsString())
	outMsgs := parseJSON(t, outVal.AsString())
	require.Len(t, inMsgs, 1)
	require.Len(t, outMsgs, 1)

	inMsg := inMsgs[0].(map[string]any)
	_, hasFinishReason := inMsg["finish_reason"]
	assert.False(t, hasFinishReason, "input messages must not have finish_reason")

	outMsg := outMsgs[0].(map[string]any)
	assert.Empty(t, outMsg["finish_reason"], "output messages must have finish_reason (empty string)")
}

func TestReconstructMessages_TrailingDotKey(t *testing.T) {
	// A key ending exactly at "message." (empty fieldPath) must not inject a
	// phantom {"role":"user","parts":[]} entry into the output.
	m := pcommon.NewMap()
	m.PutStr("llm.input_messages.0.message.", "orphan") // trailing-dot key
	m.PutStr("llm.input_messages.1.message.role", "user")
	m.PutStr("llm.input_messages.1.message.content", "hello")

	wrote := ReconstructMessages(m, true, false)
	require.True(t, wrote)

	val, ok := m.Get(otelsemconv.GenAIInputMessages)
	require.True(t, ok)
	msgs := parseJSON(t, val.AsString())
	require.Len(t, msgs, 1, "trailing-dot key must not produce a phantom message")
	assert.Equal(t, "user", msgs[0].(map[string]any)["role"])
}

func TestReconstructMessages_ToolCallIDOnOutputMessage(t *testing.T) {
	// A malformed span that puts tool_call_id on an output message must not
	// produce role:"tool", which is invalid per the GenAI output-messages schema.
	attrs := newAttrs(map[string]string{
		"llm.output_messages.0.message.content":      "result",
		"llm.output_messages.0.message.tool_call_id": "call_1",
	})

	wrote := ReconstructMessages(attrs, true, false)
	require.True(t, wrote)

	val, ok := attrs.Get(otelsemconv.GenAIOutputMessages)
	require.True(t, ok)
	msgs := parseJSON(t, val.AsString())
	require.Len(t, msgs, 1)
	role := msgs[0].(map[string]any)["role"]
	assert.NotEqual(t, "tool", role, "output messages must not have role:tool")
}

func TestMessageAggregator_Interface(t *testing.T) {
	attrs := newAttrs(map[string]string{
		"llm.input_messages.0.message.role":    "user",
		"llm.input_messages.0.message.content": "hi",
	})

	agg := MessageAggregator{}
	wrote := agg.AggregateAttributes(attrs, true, false)
	require.True(t, wrote)

	_, ok := attrs.Get(otelsemconv.GenAIInputMessages)
	assert.True(t, ok)
}
