// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package openinference holds the attribute and value mapping tables used to
// normalize OpenInference-instrumented spans to the OTel GenAI semantic
// conventions.
//
// Reference: https://github.com/Arize-ai/openinference/blob/725d68c0c43778089bc99060efba74d37231f9f1/spec/semantic_conventions.md
package openinference // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/openinference"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"
)

// LookupTable maps OpenInference attribute keys to the OTel GenAI target keys.
var LookupTable = map[string]string{
	// Token usage
	"llm.token_count.prompt":     otelsemconv.GenAIUsageInputTokens,
	"llm.token_count.completion": otelsemconv.GenAIUsageOutputTokens,

	// Model & provider
	"llm.model_name": otelsemconv.GenAIRequestModel,
	"llm.provider":   otelsemconv.GenAIProviderName,

	// Input/output content
	"llm.input_messages":  otelsemconv.GenAIInputMessages,
	"llm.output_messages": otelsemconv.GenAIOutputMessages,

	// Embeddings
	"embedding.model_name": otelsemconv.GenAIRequestModel,

	// Tool
	"tool.name":                    otelsemconv.GenAIToolName,
	"tool.description":             otelsemconv.GenAIToolDescription,
	"tool_call.function.arguments": otelsemconv.GenAIToolCallArguments,
	"tool_call.id":                 otelsemconv.GenAIToolCallID,

	// Reranker model (mutually exclusive with llm/embedding model)
	"reranker.model_name": otelsemconv.GenAIRequestModel,

	// Agent & session
	"agent.name": otelsemconv.GenAIAgentName,
	"session.id": otelsemconv.GenAIConversationID,

	// Span kind -> operation name (value normalization handled by TransformValue)
	"openinference.span.kind": otelsemconv.GenAIOperationName,
}
