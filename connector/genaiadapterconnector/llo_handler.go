// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package genaiadapterconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/genaiadapterconnector"

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/src/amazon/opentelemetry/distro/llo_handler.py#L16
const (
	roleSystem    = "system"
	roleUser      = "user"
	roleAssistant = "assistant"
	roleTool      = "tool"
)

// patternType - Types of LLO attribute patterns.
type patternType int

const (
	patternIndexed patternType = iota
	patternDirect
)

// patternConfig - Configuration for an LLO pattern.
type patternConfig struct {
	pType       patternType
	regex       string
	roleKey     string
	role        string
	defaultRole string
	source      string
}

// LLO_PATTERNS registry
var lloPatterns = map[string]patternConfig{
	"gen_ai.prompt.{index}.content": {
		pType:       patternIndexed,
		regex:       `^gen_ai\.prompt\.(\d+)\.content$`,
		roleKey:     "gen_ai.prompt.{index}.role",
		defaultRole: "unknown",
		source:      "prompt",
	},
	"gen_ai.completion.{index}.content": {
		pType:       patternIndexed,
		regex:       `^gen_ai\.completion\.(\d+)\.content$`,
		roleKey:     "gen_ai.completion.{index}.role",
		defaultRole: "unknown",
		source:      "completion",
	},
	"llm.input_messages.{index}.message.content": {
		pType:       patternIndexed,
		regex:       `^llm\.input_messages\.(\d+)\.message\.content$`,
		roleKey:     "llm.input_messages.{index}.message.role",
		defaultRole: roleUser,
		source:      "input",
	},
	"llm.output_messages.{index}.message.content": {
		pType:       patternIndexed,
		regex:       `^llm\.output_messages\.(\d+)\.message\.content$`,
		roleKey:     "llm.output_messages.{index}.message.role",
		defaultRole: roleAssistant,
		source:      "output",
	},
	"traceloop.entity.input":        {pType: patternDirect, role: roleUser, source: "input"},
	"traceloop.entity.output":       {pType: patternDirect, role: roleAssistant, source: "output"},
	"crewai.crew.tasks_output":      {pType: patternDirect, role: roleAssistant, source: "output"},
	"crewai.crew.result":            {pType: patternDirect, role: roleAssistant, source: "result"},
	"gen_ai.prompt":                 {pType: patternDirect, role: roleUser, source: "prompt"},
	"gen_ai.completion":             {pType: patternDirect, role: roleAssistant, source: "completion"},
	"gen_ai.content.revised_prompt": {pType: patternDirect, role: roleSystem, source: "prompt"},
	"gen_ai.agent.actual_output":    {pType: patternDirect, role: roleAssistant, source: "output"},
	"gen_ai.agent.human_input":      {pType: patternDirect, role: roleUser, source: "input"},
	"input.value":                   {pType: patternDirect, role: roleUser, source: "input"},
	"output.value":                  {pType: patternDirect, role: roleAssistant, source: "output"},
	"system_prompt":                 {pType: patternDirect, role: roleSystem, source: "prompt"},
	"tool.result":                   {pType: patternDirect, role: roleAssistant, source: "output"},
	"llm.prompts":                   {pType: patternDirect, role: roleUser, source: "prompt"},
	// OTel GenAI Semantic Convention used by the latest Strands SDK
	// References:
	// - OTel GenAI SemConv: https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-events/
	// - Strands SDK PR(introduced in v0.1.9): https://github.com/strands-agents/sdk-python/pull/319
	"gen_ai.user.message":        {pType: patternDirect, role: roleUser, source: "prompt"},
	"gen_ai.assistant.message":   {pType: patternDirect, role: roleAssistant, source: "output"},
	"gen_ai.system.message":      {pType: patternDirect, role: roleSystem, source: "prompt"},
	"gen_ai.tool.message":        {pType: patternDirect, role: roleTool, source: "prompt"},
	"gen_ai.choice":              {pType: patternDirect, role: roleAssistant, source: "output"},
	"gen_ai.input.messages":      {pType: patternDirect, role: roleUser, source: "input"},
	"gen_ai.output.messages":     {pType: patternDirect, role: roleAssistant, source: "output"},
	"gen_ai.system_instructions": {pType: patternDirect, role: roleSystem, source: "prompt"},
}

type compiledRegexPattern struct {
	regex      *regexp.Regexp
	patternKey string
	config     patternConfig
}

// lloHandler - Utility class for handling Large Language Objects (LLO) in OpenTelemetry spans.
//
// LLOHandler performs three primary functions:
// 1. Identifies Large Language Objects (LLO) content in spans
// 2. Extracts and transforms these attributes into OpenTelemetry Gen AI Events
// 3. Filters LLO from spans to maintain privacy and reduce span size
//
// The handler uses a configuration-driven approach with a pattern registry that defines
// all supported LLO attribute patterns and their extraction rules. This makes it easy
// to add support for new frameworks without modifying the core logic.
type lloHandler struct {
	logger             *zap.Logger
	exactMatchPatterns map[string]patternConfig
	regexPatterns      []compiledRegexPattern
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/src/amazon/opentelemetry/distro/llo_handler.py#L187
// newLLOHandler initializes an LLOHandler.
//
// This constructor sets up the event logger provider and compiles patterns
// from the pattern registry for efficient matching.
func newLLOHandler(logger *zap.Logger) *lloHandler {
	h := &lloHandler{
		logger:             logger,
		exactMatchPatterns: make(map[string]patternConfig),
	}
	h.buildPatternMatchers()
	return h
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/src/amazon/opentelemetry/distro/llo_handler.py#L203
// buildPatternMatchers - Build efficient pattern matching structures from the pattern registry.
// Creates:
// - Map of exact match patterns for O(1) lookups
// - Slice of compiled regex patterns for indexed patterns
func (h *lloHandler) buildPatternMatchers() {
	for patternKey, config := range lloPatterns {
		switch config.pType {
		case patternDirect:
			h.exactMatchPatterns[patternKey] = config
		case patternIndexed:
			if config.regex != "" {
				compiled := regexp.MustCompile(config.regex)
				h.regexPatterns = append(h.regexPatterns, compiledRegexPattern{
					regex:      compiled,
					patternKey: patternKey,
					config:     config,
				})
			}
		}
	}
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/src/amazon/opentelemetry/distro/llo_handler.py#L579
// isLLOAttribute determines if an attribute key contains LLO content based on pattern matching.
//
// Uses the pattern registry to check if a key matches any LLO pattern.
//
// Returns true if the key matches any LLO pattern, false otherwise.
func (h *lloHandler) isLLOAttribute(key string) bool {
	if _, ok := h.exactMatchPatterns[key]; ok {
		return true
	}
	for _, rp := range h.regexPatterns {
		if rp.regex.MatchString(key) {
			return true
		}
	}
	return false
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/src/amazon/opentelemetry/distro/llo_handler.py#L225
// collectAllLLOMessages collects all LLO messages from attributes using the pattern registry.
//
// This is the main collection method that processes all patterns defined
// in the registry and extracts messages accordingly.
//
// Returns a list of message maps with "content", "role", and "source" keys.
func (h *lloHandler) collectAllLLOMessages(attributes map[string]any) []map[string]any {
	var messages []map[string]any

	if attributes == nil {
		return messages
	}

	for attrKey, value := range attributes {
		if config, ok := h.exactMatchPatterns[attrKey]; ok {
			role := config.role
			if role == "" {
				role = "unknown"
			}
			source := config.source
			if source == "" {
				source = "unknown"
			}
			messages = append(messages, map[string]any{
				"content": value,
				"role":    role,
				"source":  source,
			})
		}
	}

	indexedMessages := h.collectIndexedMessages(attributes)
	messages = append(messages, indexedMessages...)

	return messages
}

type indexedKey struct {
	patternKey string
	index      int
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/src/amazon/opentelemetry/distro/llo_handler.py#L257
// collectIndexedMessages collects messages from indexed patterns (e.g., gen_ai.prompt.0.content).
//
// Handles patterns with numeric indices and their associated role attributes.
//
// Returns a list of message maps sorted by (patternKey, index).
func (h *lloHandler) collectIndexedMessages(attributes map[string]any) []map[string]any {
	indexed := make(map[indexedKey]map[string]any)

	if attributes == nil {
		return nil
	}

	for attrKey, value := range attributes {
		for _, rp := range h.regexPatterns {
			match := rp.regex.FindStringSubmatch(attrKey)
			if match != nil {
				idx, _ := strconv.Atoi(match[1])

				role := rp.config.defaultRole
				if role == "" {
					role = "unknown"
				}
				if rp.config.roleKey != "" {
					roleKey := strings.ReplaceAll(rp.config.roleKey, "{index}", match[1])
					if roleVal, ok := attributes[roleKey]; ok {
						if roleStr, ok := roleVal.(string); ok {
							role = roleStr
						}
					}
				}

				source := rp.config.source
				if source == "" {
					source = "unknown"
				}

				indexed[indexedKey{rp.patternKey, idx}] = map[string]any{
					"content": value,
					"role":    role,
					"source":  source,
				}
				break
			}
		}
	}

	sortedKeys := make([]indexedKey, 0, len(indexed))
	for k := range indexed {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Slice(sortedKeys, func(i, j int) bool {
		if sortedKeys[i].patternKey != sortedKeys[j].patternKey {
			return sortedKeys[i].patternKey < sortedKeys[j].patternKey
		}
		return sortedKeys[i].index < sortedKeys[j].index
	})

	result := make([]map[string]any, 0, len(sortedKeys))
	for _, k := range sortedKeys {
		result = append(result, indexed[k])
	}
	return result
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/src/amazon/opentelemetry/distro/llo_handler.py#L292
// collectLLOAttributesFromSpan collects all LLO attributes from a span's attributes and events.
//
// Returns a map of all LLO attributes found in the span.
func (h *lloHandler) collectLLOAttributesFromSpan(span ptrace.Span) map[string]any {
	allLLOAttributes := make(map[string]any)

	// Collect from span attributes
	span.Attributes().Range(func(key string, value pcommon.Value) bool {
		if h.isLLOAttribute(key) {
			allLLOAttributes[key] = value.AsRaw()
		}
		return true
	})

	// Collect from span events
	for i := 0; i < span.Events().Len(); i++ {
		event := span.Events().At(i)

		// Check if event name itself is an LLO pattern (e.g., "gen_ai.user.message")
		if h.isLLOAttribute(event.Name()) {
			// Put all event attributes as the content as LLO in log event
			eventAttrs := make(map[string]any)
			event.Attributes().Range(func(k string, v pcommon.Value) bool {
				eventAttrs[k] = v.AsRaw()
				return true
			})
			allLLOAttributes[event.Name()] = eventAttrs
		}

		// Also check traditional pattern - LLO attributes within event attributes
		event.Attributes().Range(func(key string, value pcommon.Value) bool {
			if h.isLLOAttribute(key) {
				allLLOAttributes[key] = value.AsRaw()
			}
			return true
		})
	}

	return allLLOAttributes
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/src/amazon/opentelemetry/distro/llo_handler.py#L327
// removeLLOAttributes removes LLO attributes from span attributes in-place.
func (h *lloHandler) removeLLOAttributes(span ptrace.Span) {
	span.Attributes().RemoveIf(func(key string, _ pcommon.Value) bool {
		return h.isLLOAttribute(key)
	})
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/src/amazon/opentelemetry/distro/llo_handler.py#L438
// groupMessagesByType groups messages into input and output categories based on role and source.
//
// Returns a map with "input" and "output" lists of messages.
func (h *lloHandler) groupMessagesByType(messages []map[string]any) map[string][]map[string]any {
	var inputMessages []map[string]any
	var outputMessages []map[string]any

	for _, message := range messages {
		role, _ := message["role"].(string)
		if role == "" {
			role = "unknown"
		}
		content := message["content"]
		if content == nil {
			content = ""
		}
		formatted := map[string]any{"role": role, "content": content}

		switch role {
		case roleSystem, roleUser:
			inputMessages = append(inputMessages, formatted)
		case roleAssistant:
			outputMessages = append(outputMessages, formatted)
		default:
			// Route based on source for non-standard roles including tool
			source, _ := message["source"].(string)
			if strings.Contains(source, "completion") || strings.Contains(source, "output") || strings.Contains(source, "result") {
				outputMessages = append(outputMessages, formatted)
			} else {
				inputMessages = append(inputMessages, formatted)
			}
		}
	}

	return map[string][]map[string]any{"input": inputMessages, "output": outputMessages}
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/src/amazon/opentelemetry/distro/llo_handler.py#L469
// emitLLOAttributes extracts LLO attributes and emits them as a single consolidated Gen AI Event.
//
// This method:
// 1. Collects all LLO attributes using the pattern registry
// 2. Groups them into input and output messages
// 3. Emits one event per span containing all LLO content
//
// The event body format:
//
//	{
//	    "input": {
//	        "messages": [
//	            {"role": "system", "content": "..."},
//	            {"role": "user", "content": "..."}
//	        ]
//	    },
//	    "output": {
//	        "messages": [
//	            {"role": "assistant", "content": "..."}
//	        ]
//	    }
//	}
func (h *lloHandler) emitLLOAttributes(
	span ptrace.Span,
	resource pcommon.Resource,
	scope pcommon.InstrumentationScope,
	attributes map[string]any,
	eventTimestamp *pcommon.Timestamp,
) plog.Logs {
	logs := plog.NewLogs()

	if attributes == nil {
		return logs
	}
	hasLLOAttrs := false
	for key := range attributes {
		if h.isLLOAttribute(key) {
			hasLLOAttrs = true
			break
		}
	}
	if !hasLLOAttrs {
		return logs
	}

	allMessages := h.collectAllLLOMessages(attributes)
	if len(allMessages) == 0 {
		return logs
	}

	// Group messages into input/output categories
	groupedMessages := h.groupMessagesByType(allMessages)

	// Build event body
	eventBody := pcommon.NewMap()
	if inputMsgs := groupedMessages["input"]; len(inputMsgs) > 0 {
		inputMap := eventBody.PutEmptyMap("input")
		msgSlice := inputMap.PutEmptySlice("messages")
		for _, msg := range inputMsgs {
			m := msgSlice.AppendEmpty().SetEmptyMap()
			if role, ok := msg["role"].(string); ok {
				m.PutStr("role", role)
			}
			if content, ok := msg["content"].(string); ok {
				m.PutStr("content", content)
			} else {
				// content may be a dict (from event attributes) - store as string
				m.PutStr("content", formatContent(msg["content"]))
			}
		}
	}
	if outputMsgs := groupedMessages["output"]; len(outputMsgs) > 0 {
		outputMap := eventBody.PutEmptyMap("output")
		msgSlice := outputMap.PutEmptySlice("messages")
		for _, msg := range outputMsgs {
			m := msgSlice.AppendEmpty().SetEmptyMap()
			if role, ok := msg["role"].(string); ok {
				m.PutStr("role", role)
			}
			if content, ok := msg["content"].(string); ok {
				m.PutStr("content", content)
			} else {
				m.PutStr("content", formatContent(msg["content"]))
			}
		}
	}

	if eventBody.Len() == 0 {
		return logs
	}

	timestamp := span.EndTimestamp()
	if eventTimestamp != nil {
		timestamp = *eventTimestamp
	}

	rl := logs.ResourceLogs().AppendEmpty()
	resource.CopyTo(rl.Resource())
	sl := rl.ScopeLogs().AppendEmpty()
	scope.CopyTo(sl.Scope())

	lr := sl.LogRecords().AppendEmpty()
	lr.SetTimestamp(timestamp)
	lr.SetObservedTimestamp(timestamp)
	lr.SetTraceID(span.TraceID())
	lr.SetSpanID(span.SpanID())
	lr.SetFlags(plog.LogRecordFlags(span.Flags()))

	lr.Attributes().PutStr("event.name", scope.Name())

	if sessionID, ok := span.Attributes().Get("session.id"); ok {
		lr.Attributes().PutStr("session.id", sessionID.AsString())
	}

	eventBody.CopyTo(lr.Body().SetEmptyMap())

	h.logger.Debug("Emitted Gen AI Event with input/output message format")

	return logs
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/src/amazon/opentelemetry/distro/llo_handler.py#L546
// filterAttributes creates a new attributes map with LLO attributes removed.
//
// This method creates a new map containing only non-LLO attributes,
// preserving the original values while filtering out sensitive LLO content.
// This helps maintain privacy and reduces the size of spans.
//
// Returns a new map with LLO attributes removed, or nil if input is nil.

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/src/amazon/opentelemetry/distro/llo_handler.py#L393
// filterSpanEvents filters LLO attributes from span events.
//
// This method removes LLO attributes from event attributes while preserving
// the event structure and non-LLO attributes.
//
// The span is modified in-place.
func (h *lloHandler) filterSpanEvents(span ptrace.Span) {
	events := span.Events()
	if events.Len() == 0 {
		return
	}

	type keepEvent struct {
		idx     int
		changed bool
		remove  []string
	}

	var kept []keepEvent
	for i := 0; i < events.Len(); i++ {
		event := events.At(i)

		// Skip entire event if event name is an LLO pattern
		if h.isLLOAttribute(event.Name()) {
			continue
		}

		if event.Attributes().Len() == 0 {
			kept = append(kept, keepEvent{idx: i})
			continue
		}

		var toRemove []string
		event.Attributes().Range(func(key string, _ pcommon.Value) bool {
			if h.isLLOAttribute(key) {
				toRemove = append(toRemove, key)
			}
			return true
		})

		if len(toRemove) > 0 {
			kept = append(kept, keepEvent{idx: i, changed: true, remove: toRemove})
		} else {
			kept = append(kept, keepEvent{idx: i})
		}
	}

	if len(kept) == events.Len() {
		// Only need to remove attrs from changed events, no events were dropped
		for _, ke := range kept {
			if ke.changed {
				for _, k := range ke.remove {
					events.At(ke.idx).Attributes().Remove(k)
				}
			}
		}
		return
	}

	// Some events were dropped - rebuild the events slice
	newEvents := ptrace.NewSpanEventSlice()
	for _, ke := range kept {
		src := events.At(ke.idx)
		if ke.changed {
			for _, k := range ke.remove {
				src.Attributes().Remove(k)
			}
		}
		src.CopyTo(newEvents.AppendEmpty())
	}
	newEvents.CopyTo(events)
}

// https://github.com/aws-observability/aws-otel-python-instrumentation/blob/35ef26e79e3ebf0253dc2f1bc03b97ab483cde31/aws-opentelemetry-distro/src/amazon/opentelemetry/distro/llo_handler.py#L345
// processSpans processes a batch of traces to extract and filter LLO attributes.
//
// For each span, this method:
// 1. Collects all LLO attributes from span attributes and all span events
// 2. Emits a single consolidated Gen AI Event with all collected LLO content
// 3. Filters out LLO attributes from the span and its events to maintain privacy
// 4. Preserves non-LLO attributes in the span
//
// Handles LLO attributes from multiple frameworks:
//   - Traceloop (indexed prompt/completion patterns and entity input/output)
//   - OpenLit (direct prompt/completion patterns, including from span events)
//   - OpenInference (input/output values and structured messages)
//   - Strands SDK (system prompts and tool results)
//   - CrewAI (tasks output and results)
//
// Returns the accumulated plog.Logs containing all emitted events.
// The input ptrace.Traces is modified in-place with LLO attributes removed.
func (h *lloHandler) processSpans(td ptrace.Traces) plog.Logs {
	allLogs := plog.NewLogs()

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)

				// 1. Collect all LLO attributes from both span attributes and events
				allLLOAttributes := h.collectLLOAttributesFromSpan(span)

				// 2. Emit a single consolidated event if we found any LLO attributes
				if len(allLLOAttributes) > 0 {
					logs := h.emitLLOAttributes(span, rs.Resource(), ss.Scope(), allLLOAttributes, nil)
					if logs.LogRecordCount() > 0 {
						logs.ResourceLogs().MoveAndAppendTo(allLogs.ResourceLogs())
					}
				}

				// 3. Remove LLO attributes from span
				h.removeLLOAttributes(span)

				// 4. Filter span events
				h.filterSpanEvents(span)
			}
		}
	}

	return allLogs
}

func formatContent(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(data)
}
