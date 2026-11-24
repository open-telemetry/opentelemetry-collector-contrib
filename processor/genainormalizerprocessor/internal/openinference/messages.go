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

// pendingKey is a source attribute whose removal is decided after the whole
// message has been read and the winning part source is known.
type pendingKey struct {
	key        string
	kind       fieldKind
	contentIdx int
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
	pending := make(map[int][]pendingKey)

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

		if kind, contentIdx, ok := applyField(mf, fieldPath, v); ok && removeOriginals {
			pending[idx] = append(pending[idx], pendingKey{key: k, kind: kind, contentIdx: contentIdx})
		}
		return true
	})

	if len(messages) == 0 {
		return false
	}

	// An attribute is removed only if the parts actually emitted for its message
	// were built from it, so nothing is dropped without being represented.
	var keysToRemove []string
	for msgIdx, keys := range pending {
		mf := messages[msgIdx]
		src := selectPartSource(mf)
		for _, pk := range keys {
			if consumedBy(pk.kind, src) && (pk.kind != kindContents || isTextContent(mf.contents[pk.contentIdx])) {
				keysToRemove = append(keysToRemove, pk.key)
			}
		}
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
	before, after, ok := strings.Cut(s, ".")
	if !ok {
		return 0, "", false
	}
	idx, err := strconv.Atoi(before)
	if err != nil {
		return 0, "", false
	}
	rest := after
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

// fieldKind groups a flattened attribute by the part source that renders it.
type fieldKind int

const (
	kindUnmapped fieldKind = iota
	kindAlways             // role, name: rendered on every message
	kindContent
	kindToolCallID
	kindToolCall
	kindContents
)

// consumedBy reports whether a field of this kind is represented in parts built
// from src. message.content is read both as a text part and as the response of a
// tool_call_response.
func consumedBy(kind fieldKind, src partSource) bool {
	switch kind {
	case kindAlways:
		return true
	case kindContent:
		return src == sourceFlatContent || src == sourceToolCallResponse
	case kindToolCallID:
		return src == sourceToolCallResponse
	case kindToolCall:
		return src == sourceToolCalls
	case kindContents:
		return src == sourceContents
	}
	return false
}

// applyField folds one flattened attribute into mf and classifies it. The bool
// reports whether the field was recognized at all; unrecognized fields keep
// their source attributes.
func applyField(mf *messageFields, fieldPath string, v pcommon.Value) (kind fieldKind, contentIdx int, ok bool) {
	switch {
	case fieldPath == "role":
		mf.role = v.AsString()
		return kindAlways, 0, true
	case fieldPath == "name":
		mf.name = v.AsString()
		return kindAlways, 0, true
	case fieldPath == "content":
		mf.content = v.AsString()
		return kindContent, 0, true
	case fieldPath == "tool_call_id":
		mf.toolCallID = v.AsString()
		return kindToolCallID, 0, true
	case strings.HasPrefix(fieldPath, "tool_calls."):
		return kindToolCall, 0, parseToolCallField(mf, fieldPath[len("tool_calls."):], v)
	case strings.HasPrefix(fieldPath, "contents."):
		idx, recognized := parseContentField(mf, fieldPath[len("contents."):], v)
		return kindContents, idx, recognized
	}
	return kindUnmapped, 0, false
}

// parseContentField parses "M.message_content.field" from the indexed content
// array into mf.contents[M]. It returns the content index and whether the field
// was recognized. Unrecognized fields (image, audio, data, signature) are left
// untouched so their source attributes survive.
func parseContentField(mf *messageFields, s string, v pcommon.Value) (int, bool) {
	before, rest, ok := strings.Cut(s, ".")
	if !ok {
		return 0, false
	}
	idx, err := strconv.Atoi(before)
	if err != nil {
		return 0, false
	}
	const mcPrefix = "message_content."
	if !strings.HasPrefix(rest, mcPrefix) {
		return 0, false
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
	default:
		return idx, false
	}
	return idx, true
}

// isTextContent reports whether a content entry is reconstructed as a text part.
// An entry with text but no declared type counts as text, because
// message_content.type is not set by every instrumentor.
func isTextContent(cf *contentFields) bool {
	return cf.text != "" && (cf.typ == "" || cf.typ == "text")
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
		if !isTextContent(cf) {
			continue
		}
		parts = append(parts, textPart{Type: "text", Content: cf.text})
	}
	return parts
}

// parseToolCallField parses "M.tool_call.field" into mf.toolCalls[M] and reports
// whether the field was recognized. Unrecognized fields (for instance
// tool_call.reasoning_signature) are left untouched so their source attributes
// survive.
func parseToolCallField(mf *messageFields, s string, v pcommon.Value) bool {
	before, after, ok := strings.Cut(s, ".")
	if !ok {
		return false
	}
	idx, err := strconv.Atoi(before)
	if err != nil {
		return false
	}
	rest := after
	const tcPrefix = "tool_call."
	if !strings.HasPrefix(rest, tcPrefix) {
		return false
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
	default:
		return false
	}
	return true
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

// partSource identifies which group of fields a message's parts are built from.
// buildParts renders it and reconstructPrefix decides removals from it, so the
// precedence rules are stated once.
type partSource int

const (
	sourceNone partSource = iota
	sourceToolCallResponse
	sourceToolCalls
	sourceFlatContent
	sourceContents
)

func selectPartSource(mf *messageFields) partSource {
	switch {
	case mf.toolCallID != "":
		return sourceToolCallResponse
	case len(mf.toolCalls) > 0:
		return sourceToolCalls
	case mf.content != "":
		return sourceFlatContent
	}
	// The flat message.content string takes precedence over the indexed array
	// when both are present: it is the field the GenAI schema maps onto directly.
	for _, cf := range mf.contents {
		if isTextContent(cf) {
			return sourceContents
		}
	}
	return sourceNone
}

func buildParts(mf *messageFields) []any {
	switch selectPartSource(mf) {
	case sourceToolCallResponse:
		return []any{
			toolCallResponsePart{
				Type:     "tool_call_response",
				ID:       mf.toolCallID,
				Response: mf.content,
			},
		}

	case sourceToolCalls:
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

	case sourceFlatContent:
		return []any{textPart{Type: "text", Content: mf.content}}

	case sourceContents:
		return textPartsFromContents(mf)
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
