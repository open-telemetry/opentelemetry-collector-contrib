// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package openllmetry holds the attribute and value mapping tables used to
// normalize OpenLLMetry (Traceloop) instrumented spans to the OTel GenAI
// semantic conventions.
//
// Reference: https://github.com/traceloop/openllmetry/blob/1ebfd1b77cfcfede74a40f28dbb0d9709bcff365/packages/opentelemetry-semantic-conventions-ai/opentelemetry/semconv_ai/__init__.py
package openllmetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/openllmetry"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"
)

// LookupTable maps OpenLLMetry attribute keys to the OTel GenAI target keys.
var LookupTable = map[string]string{
	// Token usage
	"llm.usage.prompt_tokens":     otelsemconv.GenAIUsageInputTokens,
	"llm.usage.completion_tokens": otelsemconv.GenAIUsageOutputTokens,

	// Model
	"llm.request.model":  otelsemconv.GenAIRequestModel,
	"llm.response.model": otelsemconv.GenAIResponseModel,

	// Request params
	"llm.request.max_tokens":  otelsemconv.GenAIRequestMaxTokens,
	"llm.request.temperature": otelsemconv.GenAIRequestTemperature,
	"llm.request.top_p":       otelsemconv.GenAIRequestTopP,
	"llm.top_k":               otelsemconv.GenAIRequestTopK,
	"llm.frequency_penalty":   otelsemconv.GenAIRequestFrequencyPenalty,
	"llm.presence_penalty":    otelsemconv.GenAIRequestPresencePenalty,
	"llm.chat.stop_sequences": otelsemconv.GenAIRequestStopSequences,
	"llm.request.functions":   otelsemconv.GenAIToolDefinitions,

	// Finish reason: source is a single string, target is a string[].
	// Wrapping is handled by Transformer.
	"llm.response.finish_reason": otelsemconv.GenAIResponseFinishReasons,
	"llm.response.stop_reason":   otelsemconv.GenAIResponseFinishReasons,

	// Operation: llm.request.type on LLM spans, traceloop.span.kind on workflow
	// spans. Value normalization handled by Transformer.
	"llm.request.type":    otelsemconv.GenAIOperationName,
	"traceloop.span.kind": otelsemconv.GenAIOperationName,

	// Traceloop workflow/entity (agentic)
	"traceloop.entity.name":   otelsemconv.GenAIAgentName,
	"traceloop.entity.input":  otelsemconv.GenAIInputMessages,
	"traceloop.entity.output": otelsemconv.GenAIOutputMessages,
}
