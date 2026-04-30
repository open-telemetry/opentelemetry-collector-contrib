// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package genainormalizerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor"

import "strings"

const targetOperationName = "gen_ai.operation.name"

// operationNameValues maps source operation/span-kind values to OTel GenAI
// operation names. Keys are lowercased; transformValue lowercases the input
// before lookup.
//
// Sources:
//
//	OpenInference: https://github.com/Arize-ai/openinference/blob/main/spec/semantic_conventions.md#span-kinds
//	OpenLLMetry:   https://github.com/traceloop/openllmetry/blob/main/packages/opentelemetry-semantic-conventions-ai/opentelemetry/semconv_ai/__init__.py
//	OTel GenAI:    https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-spans/
var operationNameValues = map[string]string{
	// OpenInference openinference.span.kind (original: uppercase)
	"llm":       "chat",
	"embedding": "embeddings",
	"chain":     "invoke_agent",
	"retriever": "retrieval",
	"reranker":  "retrieval",
	"tool":      "execute_tool",
	"agent":     "invoke_agent",
	"prompt":    "text_completion",

	// OpenLLMetry traceloop.span.kind (original: lowercase).
	// "agent" and "tool" overlap with OpenInference above; values agree.
	"workflow": "invoke_workflow",
	"task":     "invoke_agent",

	// OpenLLMetry llm.request.type (original: lowercase).
	// "embedding" overlaps with OpenInference EMBEDDING above; values agree.
	"completion": "text_completion",
	"chat":       "chat",
	"rerank":     "retrieval",
}

// transformValue applies value mapping for a given target attribute key.
// Returns the transformed value, or the original if no mapping exists.
func transformValue(targetKey, value string) string {
	if targetKey != targetOperationName {
		return value
	}
	if mapped, ok := operationNameValues[strings.ToLower(value)]; ok {
		return mapped
	}
	return value
}
