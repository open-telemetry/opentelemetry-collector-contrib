// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openinference // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/openinference"

import (
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"
)

// operationNameValues maps OpenInference openinference.span.kind values to
// OTel GenAI operation names. Keys are lowercased; TransformValue lowercases
// the input before lookup.
var operationNameValues = map[string]string{
	"llm":       "chat",
	"embedding": "embeddings",
	"chain":     "invoke_agent",
	"retriever": "retrieval",
	"reranker":  "retrieval",
	"tool":      "execute_tool",
	"agent":     "invoke_agent",
	"prompt":    "text_completion",
}

// Transformer adapts the package-level TransformValue function to the
// processor's valueTransformer interface.
type Transformer struct{}

// TransformValue applies OpenInference-specific value normalization when the
// target attribute is gen_ai.operation.name. Returns the original value if no
// mapping applies.
func (Transformer) TransformValue(targetKey, value string) string {
	if targetKey != otelsemconv.GenAIOperationName {
		return value
	}
	if mapped, ok := operationNameValues[strings.ToLower(value)]; ok {
		return mapped
	}
	return value
}
