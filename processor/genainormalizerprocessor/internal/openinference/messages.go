// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openinference // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/openinference"

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	oisemconv "github.com/Arize-ai/openinference/go/openinference-semantic-conventions"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"
)

type messagePrefix struct {
	prefix   string
	target   string
	isOutput bool
}

var messagePrefixes = []messagePrefix{
	{oisemconv.LLMInputMessages + ".", otelsemconv.GenAIInputMessages, false},
	{oisemconv.LLMOutputMessages + ".", otelsemconv.GenAIOutputMessages, true},
}

type inputChatMessage struct {
	Role  string `json:"role"`
	Name  string `json:"name,omitempty"`
	Parts []any  `json:"parts"`
}

// outputChatMessage mirrors inputChatMessage but adds finish_reason, which is
// required by the GenAI output-messages JSON schema. OpenInference does not
// carry per-message finish reasons, so the field is always emitted as "".
type outputChatMessage struct {
	Role         string `json:"role"`
	Name         string `json:"name,omitempty"`
	Parts        []any  `json:"parts"`
	FinishReason string `json:"finish_reason"`
}

type textPart struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type toolCallRequestPart struct {
	Type      string `json:"type"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name"`
	Arguments any    `json:"arguments,omitempty"`
}

type toolCallResponsePart struct {
	Type     string `json:"type"`
	ID       string `json:"id,omitempty"`
	Response string `json:"response"`
}

type toolCallFields struct {
	id        string
	name      string
	arguments string
}

type contentFields struct {
	typ  string
	text string
}

type messageFields struct {
	role       string
	content    string
	name       string
	toolCallID string
	toolCalls  map[int]*toolCallFields
	contents   map[int]*contentFields
}

// ReconstructMessages scans attrs for OpenInference flattened message attributes
// and reconstructs them into GenAI semconv JSON strings.
func ReconstructMessages(attrs pcommon.Map, removeOriginals, overwrite bool) bool {
	wrote := false
	for _, mp := range messagePrefixes {
		if reconstructPrefix(attrs, mp.prefix, mp.target, mp.isOutput, removeOriginals, overwrite) {
			wrote = true
		}
	}
	return wrote
}

func reconstructPrefix(attrs pcommon.Map, prefix, target string, isOutput, removeOriginals, overwrite bool) bool {
	if _, existed := attrs.Get(target); existed && !overwrite {
		return false
	}

	messages := make(map[int]*messageFields)
	var keysToRemove []string

	attrs.Range(func(k string, v pcommon.Value) bool {
		if !strings.HasPrefix(k, prefix) {
			return true
		}
		rest := k[len(prefix):]
		idx, fieldPath, ok := parseIndexedField(rest)
		if !ok {
			return true
		}

		mf, exists := messages[idx]
		if !exists {
			mf = &messageFields{
				toolCalls: make(map[int]*toolCallFields),
				contents:  make(map[int]*contentFields),
			}
			messages[idx] = mf
		}

		applyField(mf, fieldPath, v)
		if removeOriginals {
			keysToRemove = append(keysToRemove, k)
		}
		return true
	})

	if len(messages) == 0 {
		return false
	}

	result := buildMessages(messages, isOutput)
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return false
	}

	attrs.PutStr(target, string(jsonBytes))

	for _, k := range keysToRemove {
		attrs.Remove(k)
	}

	return true
}

// parseIndexedField splits "N.message.field.path" into (N, "field.path", true).
func parseIndexedField(s string) (int, string, bool) {
	dotIdx := strings.IndexByte(s, '.')
	if dotIdx < 0 {
		return 0, "", false
	}
	idx, err := strconv.Atoi(s[:dotIdx])
	if err != nil {
		return 0, "", false
	}
	rest := s[dotIdx+1:]
	const msgPrefix = "message."
	if !strings.HasPrefix(rest, msgPrefix) {
		return 0, "", false
	}
	fieldPath := rest[len(msgPrefix):]
	if fieldPath == "" {
		return 0, "", false
	}
	return idx, fieldPath, true
}

func applyField(mf *messageFields, fieldPath string, v pcommon.Value) {
	switch {
	case fieldPath == "role":
		mf.role = v.AsString()
	case fieldPath == "content":
		mf.content = v.AsString()
	case fieldPath == "name":
		mf.name = v.AsString()
	case fieldPath == "tool_call_id":
		mf.toolCallID = v.AsString()
	case strings.HasPrefix(fieldPath, "tool_calls."):
		parseToolCallField(mf, fieldPath[len("tool_calls."):], v)
	case strings.HasPrefix(fieldPath, "contents."):
		parseContentField(mf, fieldPath[len("contents."):], v)
	}
}

// parseContentField parses "M.message_content.field" from the indexed content
// array into mf.contents[M].
func parseContentField(mf *messageFields, s string, v pcommon.Value) {
	dotIdx := strings.IndexByte(s, '.')
	if dotIdx < 0 {
		return
	}
	idx, err := strconv.Atoi(s[:dotIdx])
	if err != nil {
		return
	}
	rest := s[dotIdx+1:]
	const mcPrefix = "message_content."
	if !strings.HasPrefix(rest, mcPrefix) {
		return
	}
	field := rest[len(mcPrefix):]

	cf, exists := mf.contents[idx]
	if !exists {
		cf = &contentFields{}
		mf.contents[idx] = cf
	}

	switch field {
	case "type":
		cf.typ = v.AsString()
	case "text":
		cf.text = v.AsString()
	}
}

// textPartsFromContents builds text parts from the indexed content array,
// preserving source order. Non-text entries (image, audio, reasoning, tool_use)
// carry their payload in fields this function does not read and are skipped;
// see the multimodal limitation in the README.
func textPartsFromContents(mf *messageFields) []any {
	indices := make([]int, 0, len(mf.contents))
	for idx := range mf.contents {
		indices = append(indices, idx)
	}
	sort.Ints(indices)

	parts := make([]any, 0, len(indices))
	for _, idx := range indices {
		cf := mf.contents[idx]
		// An entry with text but no declared type is still a text part
		// (message_content.type is not set by every instrumentor).
		if cf.text == "" || (cf.typ != "" && cf.typ != "text") {
			continue
		}
		parts = append(parts, textPart{Type: "text", Content: cf.text})
	}
	return parts
}

func parseToolCallField(mf *messageFields, s string, v pcommon.Value) {
	dotIdx := strings.IndexByte(s, '.')
	if dotIdx < 0 {
		return
	}
	idx, err := strconv.Atoi(s[:dotIdx])
	if err != nil {
		return
	}
	rest := s[dotIdx+1:]
	const tcPrefix = "tool_call."
	if !strings.HasPrefix(rest, tcPrefix) {
		return
	}
	field := rest[len(tcPrefix):]

	tc, exists := mf.toolCalls[idx]
	if !exists {
		tc = &toolCallFields{}
		mf.toolCalls[idx] = tc
	}

	switch field {
	case "id":
		tc.id = v.AsString()
	case "function.name":
		tc.name = v.AsString()
	case "function.arguments":
		tc.arguments = v.AsString()
	}
}

func buildMessages(messages map[int]*messageFields, isOutput bool) []any {
	indices := make([]int, 0, len(messages))
	for idx := range messages {
		indices = append(indices, idx)
	}
	sort.Ints(indices)

	result := make([]any, 0, len(indices))
	for _, idx := range indices {
		result = append(result, buildSingleMessage(messages[idx], isOutput))
	}
	return result
}

func buildSingleMessage(mf *messageFields, isOutput bool) any {
	role := inferRole(mf, isOutput)
	parts := buildParts(mf)

	if isOutput {
		return outputChatMessage{Role: role, Name: mf.name, Parts: parts, FinishReason: ""}
	}
	return inputChatMessage{Role: role, Name: mf.name, Parts: parts}
}

func buildParts(mf *messageFields) []any {
	if mf.toolCallID != "" {
		return []any{
			toolCallResponsePart{
				Type:     "tool_call_response",
				ID:       mf.toolCallID,
				Response: mf.content,
			},
		}
	}

	if len(mf.toolCalls) > 0 {
		tcIndices := make([]int, 0, len(mf.toolCalls))
		for idx := range mf.toolCalls {
			tcIndices = append(tcIndices, idx)
		}
		sort.Ints(tcIndices)

		parts := make([]any, 0, len(tcIndices))
		for _, idx := range tcIndices {
			tc := mf.toolCalls[idx]
			part := toolCallRequestPart{
				Type: "tool_call",
				ID:   tc.id,
				Name: tc.name,
			}
			if tc.arguments != "" {
				var parsed any
				if err := json.Unmarshal([]byte(tc.arguments), &parsed); err == nil {
					part.Arguments = parsed
				} else {
					part.Arguments = tc.arguments
				}
			}
			parts = append(parts, part)
		}
		return parts
	}

	if mf.content != "" {
		return []any{textPart{Type: "text", Content: mf.content}}
	}

	// Fall back to the indexed content array. The flat message.content string
	// takes precedence when both are present: it is the field the GenAI schema
	// maps onto directly, and instrumentors that set both keep them in sync.
	if parts := textPartsFromContents(mf); len(parts) > 0 {
		return parts
	}
	return []any{}
}

// GenAI semconv role enum values for input/output messages.
const (
	roleSystem    = "system"
	roleUser      = "user"
	roleAssistant = "assistant"
	roleTool      = "tool"
)

var validRoles = map[string]bool{
	roleSystem:    true,
	roleUser:      true,
	roleAssistant: true,
	roleTool:      true,
}

// inferRole derives the GenAI semconv role for a message. The schema permits
// the same role enum (system/user/assistant/tool) on both input and output
// messages, but a model-generated output turn is never a tool response, so
// "tool" is suppressed on output and falls through to assistant.
func inferRole(mf *messageFields, isOutput bool) string {
	if mf.toolCallID != "" && !isOutput {
		return roleTool
	}
	if validRoles[mf.role] && (!isOutput || mf.role != roleTool) {
		return mf.role
	}
	if len(mf.toolCalls) > 0 {
		return roleAssistant
	}
	return roleUser
}

// MessageAggregator implements the processor's attributeAggregator interface.
type MessageAggregator struct{}

func (MessageAggregator) AggregateAttributes(attrs pcommon.Map, removeOriginals, overwrite bool) bool {
	return ReconstructMessages(attrs, removeOriginals, overwrite)
}
