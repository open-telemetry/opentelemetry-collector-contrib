// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openinference // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/openinference"

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"
)

type messagePrefix struct {
	prefix string
	target string
}

var messagePrefixes = []messagePrefix{
	{"llm.input_messages.", otelsemconv.GenAIInputMessages},
	{"llm.output_messages.", otelsemconv.GenAIOutputMessages},
}

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
	Type   string `json:"type"`
	ID     string `json:"id,omitempty"`
	Result string `json:"result"`
}

type toolCallFields struct {
	id        string
	name      string
	arguments string
}

type messageFields struct {
	role       string
	content    string
	name       string
	toolCallID string
	toolCalls  map[int]*toolCallFields
}

// ReconstructMessages scans attrs for OpenInference flattened message attributes
// and reconstructs them into GenAI semconv JSON strings.
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

		applyField(mf, fieldPath, v)
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
	const msgPrefix = "message."
	if !strings.HasPrefix(rest, msgPrefix) {
		return 0, "", false
	}
	return idx, rest[len(msgPrefix):], true
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
	}
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

func buildMessages(messages map[int]*messageFields) []chatMessage {
	indices := make([]int, 0, len(messages))
	for idx := range messages {
		indices = append(indices, idx)
	}
	sort.Ints(indices)

	result := make([]chatMessage, 0, len(indices))
	for _, idx := range indices {
		result = append(result, buildSingleMessage(messages[idx]))
	}
	return result
}

func buildSingleMessage(mf *messageFields) chatMessage {
	msg := chatMessage{Role: inferRole(mf)}

	if mf.toolCallID != "" {
		msg.Parts = []interface{}{
			toolCallResponsePart{
				Type:   "tool_call_response",
				ID:     mf.toolCallID,
				Result: mf.content,
			},
		}
		return msg
	}

	if len(mf.toolCalls) > 0 {
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

func inferRole(mf *messageFields) string {
	if mf.toolCallID != "" {
		if mf.role == "" || mf.role == "user" {
			return "tool"
		}
		return mf.role
	}
	if mf.role != "" {
		return mf.role
	}
	if len(mf.toolCalls) > 0 {
		return "assistant"
	}
	return "user"
}

// MessageAggregator implements the processor's attributeAggregator interface.
type MessageAggregator struct{}

func (MessageAggregator) AggregateAttributes(attrs pcommon.Map, removeOriginals, overwrite bool) bool {
	return ReconstructMessages(attrs, removeOriginals, overwrite)
}
