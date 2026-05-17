// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openinference // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/openinference"

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"
)

// messagePrefix pairs an OpenInference flattened attribute prefix with the
// GenAI semconv target key it should be reconstructed into.
type messagePrefix struct {
	prefix string
	target string
}

var messagePrefixes = []messagePrefix{
	{"llm.input_messages.", otelsemconv.GenAIInputMessages},
	{"llm.output_messages.", otelsemconv.GenAIOutputMessages},
}

// chatMessage is the GenAI semconv ChatMessage JSON structure.
type chatMessage struct {
	Role  string        `json:"role"`
	Parts []interface{} `json:"parts"`
}

type textPart struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type toolCallRequestPart struct {
	Type      string      `json:"type"`
	ID        string      `json:"id,omitempty"`
	Name      string      `json:"name"`
	Arguments interface{} `json:"arguments,omitempty"`
}

type toolCallResponsePart struct {
	Type     string `json:"type"`
	ID       string `json:"id,omitempty"`
	Response string `json:"response"`
}

// toolCallFields collects the parts of a single tool call from flattened attrs.
type toolCallFields struct {
	id        string
	name      string
	arguments string
}

// messageFields collects all parsed fields for a single message.
type messageFields struct {
	role       string
	content    string
	toolCallID string
	toolCalls  map[int]*toolCallFields
}

// ReconstructMessages scans attrs for OpenInference flattened message attributes
// (llm.input_messages.N.message.* and llm.output_messages.N.message.*),
// reconstructs them into GenAI semconv JSON, and sets the target attributes.
// Returns true if any attributes were written.
func ReconstructMessages(attrs pcommon.Map, removeOriginals, overwrite bool) bool {
	wrote := false
	for _, mp := range messagePrefixes {
		if reconstructPrefix(attrs, mp.prefix, mp.target, removeOriginals, overwrite) {
			wrote = true
		}
	}
	return wrote
}

func reconstructPrefix(attrs pcommon.Map, prefix, target string, removeOriginals, overwrite bool) bool {
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
			mf = &messageFields{toolCalls: make(map[int]*toolCallFields)}
			messages[idx] = mf
		}

		applyField(mf, fieldPath, v.AsString())
		keysToRemove = append(keysToRemove, k)
		return true
	})

	if len(messages) == 0 {
		return false
	}

	if _, existed := attrs.Get(target); existed && !overwrite {
		return false
	}

	result := buildMessages(messages)
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return false
	}

	attrs.PutStr(target, string(jsonBytes))

	if removeOriginals {
		for _, k := range keysToRemove {
			attrs.Remove(k)
		}
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
	// Strip the "message." prefix that OpenInference always includes
	const messagePrefix = "message."
	if !strings.HasPrefix(rest, messagePrefix) {
		return 0, "", false
	}
	return idx, rest[len(messagePrefix):], true
}

// applyField sets the appropriate field on messageFields based on the field path.
func applyField(mf *messageFields, fieldPath, value string) {
	switch {
	case fieldPath == "role":
		mf.role = value
	case fieldPath == "content":
		mf.content = value
	case fieldPath == "tool_call_id":
		mf.toolCallID = value
	case strings.HasPrefix(fieldPath, "tool_calls."):
		parseToolCallField(mf, fieldPath[len("tool_calls."):], value)
	}
}

// parseToolCallField parses "M.tool_call.{id|function.name|function.arguments}".
func parseToolCallField(mf *messageFields, s, value string) {
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
		tc.id = value
	case "function.name":
		tc.name = value
	case "function.arguments":
		tc.arguments = value
	}
}

// buildMessages converts the parsed message map into a sorted slice of
// GenAI semconv ChatMessage objects.
func buildMessages(messages map[int]*messageFields) []chatMessage {
	indices := make([]int, 0, len(messages))
	for idx := range messages {
		indices = append(indices, idx)
	}
	sort.Ints(indices)

	result := make([]chatMessage, 0, len(indices))
	for _, idx := range indices {
		mf := messages[idx]
		msg := buildSingleMessage(mf)
		result = append(result, msg)
	}
	return result
}

func buildSingleMessage(mf *messageFields) chatMessage {
	msg := chatMessage{Role: mf.role}

	if mf.toolCallID != "" {
		// Tool result message
		if mf.role == "user" {
			msg.Role = "tool"
		}
		msg.Parts = []interface{}{
			toolCallResponsePart{
				Type:     "tool_call_response",
				ID:       mf.toolCallID,
				Response: mf.content,
			},
		}
		return msg
	}

	if len(mf.toolCalls) > 0 {
		// Assistant message with tool calls
		tcIndices := make([]int, 0, len(mf.toolCalls))
		for idx := range mf.toolCalls {
			tcIndices = append(tcIndices, idx)
		}
		sort.Ints(tcIndices)

		parts := make([]interface{}, 0, len(tcIndices))
		for _, idx := range tcIndices {
			tc := mf.toolCalls[idx]
			part := toolCallRequestPart{
				Type: "tool_call",
				ID:   tc.id,
				Name: tc.name,
			}
			if tc.arguments != "" {
				var parsed interface{}
				if err := json.Unmarshal([]byte(tc.arguments), &parsed); err == nil {
					part.Arguments = parsed
				} else {
					part.Arguments = tc.arguments
				}
			}
			parts = append(parts, part)
		}
		msg.Parts = parts
		return msg
	}

	if mf.content != "" {
		msg.Parts = []interface{}{
			textPart{Type: "text", Content: mf.content},
		}
	} else {
		msg.Parts = []interface{}{}
	}

	return msg
}

// MessageAggregator implements the processor's attributeAggregator interface
// for OpenInference flattened message attributes.
type MessageAggregator struct{}

// AggregateAttributes reconstructs flattened OpenInference message attributes
// into GenAI semconv JSON message strings.
func (MessageAggregator) AggregateAttributes(attrs pcommon.Map, removeOriginals, overwrite bool) bool {
	return ReconstructMessages(attrs, removeOriginals, overwrite)
}

// FormatMessageKey is a helper for tests to construct flattened attribute keys.
func FormatMessageKey(direction string, idx int, field string) string {
	return fmt.Sprintf("llm.%s_messages.%d.message.%s", direction, idx, field)
}
