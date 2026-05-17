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

func newAttrs() pcommon.Map {
	return pcommon.NewMap()
}

func TestReconstructMessages_SimpleUserAssistant(t *testing.T) {
	attrs := newAttrs()
	attrs.PutStr("llm.input_messages.0.message.role", "user")
	attrs.PutStr("llm.input_messages.0.message.content", "Hello")
	attrs.PutStr("llm.input_messages.1.message.role", "assistant")
	attrs.PutStr("llm.input_messages.1.message.content", "Hi there!")

	wrote := ReconstructMessages(attrs, true, false)
	require.True(t, wrote)

	v, ok := attrs.Get(otelsemconv.GenAIInputMessages)
	require.True(t, ok)

	var messages []chatMessage
	require.NoError(t, json.Unmarshal([]byte(v.Str()), &messages))
	require.Len(t, messages, 2)

	assert.Equal(t, "user", messages[0].Role)
	assert.Equal(t, "assistant", messages[1].Role)

	// Verify content parts
	part0 := messages[0].Parts[0].(map[string]interface{})
	assert.Equal(t, "text", part0["type"])
	assert.Equal(t, "Hello", part0["content"])

	// Originals removed
	_, ok = attrs.Get("llm.input_messages.0.message.role")
	assert.False(t, ok)
}

func TestReconstructMessages_ToolCallFlow(t *testing.T) {
	attrs := newAttrs()
	// User message
	attrs.PutStr("llm.input_messages.0.message.role", "user")
	attrs.PutStr("llm.input_messages.0.message.content", "What is the weather?")
	// Assistant message with tool call
	attrs.PutStr("llm.input_messages.1.message.role", "assistant")
	attrs.PutStr("llm.input_messages.1.message.tool_calls.0.tool_call.id", "call_123")
	attrs.PutStr("llm.input_messages.1.message.tool_calls.0.tool_call.function.name", "get_weather")
	attrs.PutStr("llm.input_messages.1.message.tool_calls.0.tool_call.function.arguments", `{"location":"Seattle"}`)
	// Tool result message
	attrs.PutStr("llm.input_messages.2.message.role", "user")
	attrs.PutStr("llm.input_messages.2.message.tool_call_id", "call_123")
	attrs.PutStr("llm.input_messages.2.message.content", `{"weather":"sunny"}`)

	wrote := ReconstructMessages(attrs, true, false)
	require.True(t, wrote)

	v, ok := attrs.Get(otelsemconv.GenAIInputMessages)
	require.True(t, ok)

	var messages []json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(v.Str()), &messages))
	require.Len(t, messages, 3)

	// Check tool result message has role "tool" (remapped from "user")
	var msg2 map[string]interface{}
	require.NoError(t, json.Unmarshal(messages[2], &msg2))
	assert.Equal(t, "tool", msg2["role"])
	parts := msg2["parts"].([]interface{})
	part := parts[0].(map[string]interface{})
	assert.Equal(t, "tool_call_response", part["type"])
	assert.Equal(t, "call_123", part["id"])

	// Check assistant tool call message
	var msg1 map[string]interface{}
	require.NoError(t, json.Unmarshal(messages[1], &msg1))
	assert.Equal(t, "assistant", msg1["role"])
	parts1 := msg1["parts"].([]interface{})
	tc := parts1[0].(map[string]interface{})
	assert.Equal(t, "tool_call", tc["type"])
	assert.Equal(t, "get_weather", tc["name"])
	assert.Equal(t, "call_123", tc["id"])
}

func TestReconstructMessages_OutputMessages(t *testing.T) {
	attrs := newAttrs()
	attrs.PutStr("llm.output_messages.0.message.role", "assistant")
	attrs.PutStr("llm.output_messages.0.message.content", "The answer is 42.")

	wrote := ReconstructMessages(attrs, true, false)
	require.True(t, wrote)

	v, ok := attrs.Get(otelsemconv.GenAIOutputMessages)
	require.True(t, ok)

	var messages []chatMessage
	require.NoError(t, json.Unmarshal([]byte(v.Str()), &messages))
	require.Len(t, messages, 1)
	assert.Equal(t, "assistant", messages[0].Role)
}

func TestReconstructMessages_NoMatchReturnsNoWrite(t *testing.T) {
	attrs := newAttrs()
	attrs.PutStr("http.method", "GET")

	wrote := ReconstructMessages(attrs, true, false)
	assert.False(t, wrote)
}

func TestReconstructMessages_OverwriteFalseSkipsExisting(t *testing.T) {
	attrs := newAttrs()
	attrs.PutStr(otelsemconv.GenAIInputMessages, "existing")
	attrs.PutStr("llm.input_messages.0.message.role", "user")
	attrs.PutStr("llm.input_messages.0.message.content", "Hello")

	wrote := ReconstructMessages(attrs, false, false)
	assert.False(t, wrote)

	v, _ := attrs.Get(otelsemconv.GenAIInputMessages)
	assert.Equal(t, "existing", v.Str())
}

func TestReconstructMessages_OverwriteTrueReplacesExisting(t *testing.T) {
	attrs := newAttrs()
	attrs.PutStr(otelsemconv.GenAIInputMessages, "existing")
	attrs.PutStr("llm.input_messages.0.message.role", "user")
	attrs.PutStr("llm.input_messages.0.message.content", "Hello")

	wrote := ReconstructMessages(attrs, false, true)
	require.True(t, wrote)

	v, _ := attrs.Get(otelsemconv.GenAIInputMessages)
	assert.NotEqual(t, "existing", v.Str())
}

func TestReconstructMessages_RemoveOriginalsFalseKeepsSources(t *testing.T) {
	attrs := newAttrs()
	attrs.PutStr("llm.input_messages.0.message.role", "user")
	attrs.PutStr("llm.input_messages.0.message.content", "Hello")

	wrote := ReconstructMessages(attrs, false, false)
	require.True(t, wrote)

	_, ok := attrs.Get("llm.input_messages.0.message.role")
	assert.True(t, ok)
}

func TestReconstructMessages_MultipleToolCalls(t *testing.T) {
	attrs := newAttrs()
	attrs.PutStr("llm.input_messages.0.message.role", "assistant")
	attrs.PutStr("llm.input_messages.0.message.tool_calls.0.tool_call.id", "call_1")
	attrs.PutStr("llm.input_messages.0.message.tool_calls.0.tool_call.function.name", "tool_a")
	attrs.PutStr("llm.input_messages.0.message.tool_calls.0.tool_call.function.arguments", `{}`)
	attrs.PutStr("llm.input_messages.0.message.tool_calls.1.tool_call.id", "call_2")
	attrs.PutStr("llm.input_messages.0.message.tool_calls.1.tool_call.function.name", "tool_b")
	attrs.PutStr("llm.input_messages.0.message.tool_calls.1.tool_call.function.arguments", `{"x":1}`)

	wrote := ReconstructMessages(attrs, true, false)
	require.True(t, wrote)

	v, _ := attrs.Get(otelsemconv.GenAIInputMessages)
	var messages []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(v.Str()), &messages))
	require.Len(t, messages, 1)

	parts := messages[0]["parts"].([]interface{})
	require.Len(t, parts, 2)
	assert.Equal(t, "tool_a", parts[0].(map[string]interface{})["name"])
	assert.Equal(t, "tool_b", parts[1].(map[string]interface{})["name"])
}

func TestFormatMessageKey(t *testing.T) {
	assert.Equal(t,
		"llm.input_messages.0.message.role",
		FormatMessageKey("input", 0, "role"),
	)
	assert.Equal(t,
		"llm.output_messages.2.message.content",
		FormatMessageKey("output", 2, "content"),
	)
}
