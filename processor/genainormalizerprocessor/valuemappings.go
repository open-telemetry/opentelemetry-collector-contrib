package genainormalizerprocessor

import "strings"

// operationNameValues maps source operation/span-kind values to OTel GenAI operation names.
// Keys preserve original casing from source specs for readability.
//
// Sources:
//
//	OpenInference: https://github.com/Arize-ai/openinference/blob/main/spec/semantic_conventions.md#span-kinds
//	OpenLLMetry:   traceloop.span.kind values from semconv_ai/__init__.py TraceloopSpanKindValues
//	OpenLLMetry:   llm.request.type values from semconv_ai/__init__.py LLMRequestTypeValues
//	OTel GenAI:    https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-spans/
var operationNameValues = map[string]string{
	// OpenInference span kinds (uppercase)
	"LLM":       "chat",
	"EMBEDDING": "embeddings",
	"CHAIN":     "invoke_agent",
	"RETRIEVER": "retrieval",
	"RERANKER":  "retrieval",
	"TOOL":      "execute_tool",
	"AGENT":     "invoke_agent",
	"PROMPT":    "text_completion",

	// OpenLLMetry traceloop.span.kind (lowercase)
	"workflow": "invoke_agent",
	"task":     "invoke_agent",
	"agent":    "invoke_agent",
	"tool":     "execute_tool",

	// OpenLLMetry llm.request.type (lowercase)
	"completion": "text_completion",
	"chat":       "chat",
	"rerank":     "retrieval",
	"embedding":  "embeddings",
}

// operationNameValuesNormalized is built at init with lowercased keys for case-insensitive lookup.
var operationNameValuesNormalized = func() map[string]string {
	m := make(map[string]string, len(operationNameValues))
	for k, v := range operationNameValues {
		m[strings.ToLower(k)] = v
	}
	return m
}()

// TransformValue applies value mapping for a given target attribute key.
// Returns the transformed value, or the original if no mapping exists.
func TransformValue(targetKey string, value string) string {
	if targetKey != "gen_ai.operation.name" {
		return value
	}
	if mapped, ok := operationNameValuesNormalized[strings.ToLower(value)]; ok {
		return mapped
	}
	return value
}
