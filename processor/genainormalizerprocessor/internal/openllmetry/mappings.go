// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package openllmetry holds the attribute and value mapping tables used to
// normalize OpenLLMetry (Traceloop) instrumented spans to the OTel GenAI
// semantic conventions. Keys are sourced from the upstream Go semconv
// package where it covers them; the rest are taken from the Python
// semconv_ai package and hard-coded with a comment.
//
// Reference: https://github.com/traceloop/openllmetry/blob/1ebfd1b77cfcfede74a40f28dbb0d9709bcff365/packages/opentelemetry-semantic-conventions-ai/opentelemetry/semconv_ai/__init__.py
package openllmetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/openllmetry"

import (
	semconvai "github.com/traceloop/go-openllmetry/semconv-ai"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"
)

// LookupTable maps OpenLLMetry attribute keys to the OTel GenAI target keys.
var LookupTable = map[string]string{
	// Token usage
	string(semconvai.LLMUsagePromptTokens):     otelsemconv.GenAIUsageInputTokens,
	string(semconvai.LLMUsageCompletionTokens): otelsemconv.GenAIUsageOutputTokens,

	// Model
	string(semconvai.LLMRequestModel):  otelsemconv.GenAIRequestModel,
	string(semconvai.LLMResponseModel): otelsemconv.GenAIResponseModel,

	// Request params
	string(semconvai.LLMRequestMaxTokens): otelsemconv.GenAIRequestMaxTokens,
	// Not in go-openllmetry/semconv-ai; defined in the canonical Python
	// semconv_ai package.
	"llm.request.temperature":             otelsemconv.GenAIRequestTemperature,
	"llm.request.top_p":                   otelsemconv.GenAIRequestTopP,
	string(semconvai.LLMTopK):             otelsemconv.GenAIRequestTopK,
	string(semconvai.LLMFrequencyPenalty): otelsemconv.GenAIRequestFrequencyPenalty,
	string(semconvai.LLMPresencePenalty):  otelsemconv.GenAIRequestPresencePenalty,
	string(semconvai.LLMChatStopSequence): otelsemconv.GenAIRequestStopSequences,
	string(semconvai.LLMRequestFunctions): otelsemconv.GenAIToolDefinitions,

	// Finish reason: source is a single string. Wrapping into a string[] at
	// gen_ai.response.finish_reasons is handled by otelsemconv.Coerce.
	// Not in go-openllmetry/semconv-ai; defined in the Python
	// semconv_ai package.
	"llm.response.finish_reason": otelsemconv.GenAIResponseFinishReasons,
	"llm.response.stop_reason":   otelsemconv.GenAIResponseFinishReasons,

	// Operation: llm.request.type on LLM spans, traceloop.span.kind on
	// workflow spans. Enum folding handled by Transform.
	string(semconvai.LLMRequestType):    otelsemconv.GenAIOperationName,
	string(semconvai.TraceloopSpanKind): otelsemconv.GenAIOperationName,

	// Traceloop workflow/entity (agentic)
	string(semconvai.TraceloopEntityName): otelsemconv.GenAIAgentName,
	// Not in go-openllmetry/semconv-ai; defined in the Python
	// semconv_ai package.
	"traceloop.entity.input":  otelsemconv.GenAIInputMessages,
	"traceloop.entity.output": otelsemconv.GenAIOutputMessages,
}
