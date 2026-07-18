// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package openinference holds the attribute and value mapping tables used to
// normalize OpenInference-instrumented spans to the OTel GenAI semantic
// conventions. Source-side keys come from the upstream Go semconv package.
//
// Reference: https://github.com/Arize-ai/openinference/blob/725d68c0c43778089bc99060efba74d37231f9f1/spec/semantic_conventions.md
package openinference // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/openinference"

import (
	oisemconv "github.com/Arize-ai/openinference/go/openinference-semantic-conventions"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"
)

// LookupTable maps OpenInference attribute keys to the OTel GenAI target keys.
var LookupTable = map[string]string{
	// Token usage
	oisemconv.LLMTokenCountPrompt:     otelsemconv.GenAIUsageInputTokens,
	oisemconv.LLMTokenCountCompletion: otelsemconv.GenAIUsageOutputTokens,

	// Model & provider
	oisemconv.LLMModelName: otelsemconv.GenAIRequestModel,
	oisemconv.LLMProvider:  otelsemconv.GenAIProviderName,

	// Flattened message attributes (llm.input_messages.N.message.*) are handled
	// by ReconstructMessages via MessageAggregator, not by simple key rename.

	// Embeddings
	oisemconv.EmbeddingModelName: otelsemconv.GenAIRequestModel,

	// Tool
	oisemconv.ToolName:                      otelsemconv.GenAIToolName,
	oisemconv.ToolDescription:               otelsemconv.GenAIToolDescription,
	oisemconv.ToolCallFunctionArgumentsJSON: otelsemconv.GenAIToolCallArguments,
	oisemconv.ToolCallID:                    otelsemconv.GenAIToolCallID,

	// Reranker model
	oisemconv.RerankerModelName: otelsemconv.GenAIRequestModel,

	// Agent & session
	oisemconv.AgentName: otelsemconv.GenAIAgentName,
	oisemconv.SessionID: otelsemconv.GenAIConversationID,

	// Span kind -> operation name (value normalization handled by Transform)
	oisemconv.OpenInferenceSpanKind: otelsemconv.GenAIOperationName,
}
