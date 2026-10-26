// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package otelsemconv pins the OTel semantic-conventions version this
// processor targets and exports the gen_ai.* attribute keys it emits, along
// with the Go type each key is defined to carry.
//
// semconv v1.42.0 moved gen_ai.* to the semantic-conventions-genai repo, so
// attribute keys, operation name enum values, and target types are defined as
// local constants and tables matching the semconv 1.40.0 definitions.
//
// Adding a new attribute: add the key constant, and if it carries an enforced
// Go type (non-"any"), register its type in targetTypes. Coerce enforces these
// types at write time. Keys whose spec type is "any" (e.g. gen_ai.input.messages)
// or an open enum not coerced (e.g. gen_ai.operation.name) are not registered;
// values for these keys pass through Coerce verbatim with no type enforcement.
package otelsemconv // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"

import (
	"reflect"

	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// SchemaURL is the OTel semconv schema URL for the targeted version.
const SchemaURL = conventions.SchemaURL

// GenAI attribute keys, grouped by subnamespace.
const (
	// gen_ai.agent.*
	GenAIAgentName = "gen_ai.agent.name"

	// gen_ai.conversation.*
	GenAIConversationID = "gen_ai.conversation.id"

	// gen_ai.input.*
	GenAIInputMessages = "gen_ai.input.messages"

	// gen_ai.operation.*
	GenAIOperationName = "gen_ai.operation.name"

	// gen_ai.output.*
	GenAIOutputMessages = "gen_ai.output.messages"

	// gen_ai.provider.*
	GenAIProviderName = "gen_ai.provider.name"

	// gen_ai.request.*
	GenAIRequestFrequencyPenalty = "gen_ai.request.frequency_penalty"
	// #nosec G101 - not hardcoded credentials
	GenAIRequestMaxTokens       = "gen_ai.request.max_tokens"
	GenAIRequestModel           = "gen_ai.request.model"
	GenAIRequestPresencePenalty = "gen_ai.request.presence_penalty"
	GenAIRequestStopSequences   = "gen_ai.request.stop_sequences"
	GenAIRequestTemperature     = "gen_ai.request.temperature"
	GenAIRequestTopK            = "gen_ai.request.top_k"
	GenAIRequestTopP            = "gen_ai.request.top_p"

	// gen_ai.response.*
	GenAIResponseFinishReasons = "gen_ai.response.finish_reasons"
	GenAIResponseModel         = "gen_ai.response.model"

	// gen_ai.tool.*
	GenAIToolCallArguments = "gen_ai.tool.call.arguments"
	GenAIToolCallID        = "gen_ai.tool.call.id"
	GenAIToolDefinitions   = "gen_ai.tool.definitions"
	GenAIToolDescription   = "gen_ai.tool.description"
	GenAIToolName          = "gen_ai.tool.name"

	// gen_ai.usage.*
	// #nosec G101 - not hardcoded credentials
	GenAIUsageInputTokens = "gen_ai.usage.input_tokens"
	// #nosec G101 - not hardcoded credentials
	GenAIUsageOutputTokens = "gen_ai.usage.output_tokens"
)

// gen_ai.operation.name enum values. Source-side normalizers (openinference,
// openllmetry) fold their own span-kind/operation-type enums onto these.
const (
	GenAIOperationNameChat           = "chat"
	GenAIOperationNameEmbeddings     = "embeddings"
	GenAIOperationNameExecuteTool    = "execute_tool"
	GenAIOperationNameInvokeAgent    = "invoke_agent"
	GenAIOperationNameRetrieval      = "retrieval"
	GenAIOperationNameTextCompletion = "text_completion"

	// GenAIOperationNameInvokeWorkflow is not part of the standard enum. It
	// is emitted by openllmetry's traceloop.span.kind=workflow path; we keep
	// it as a centralized constant so callers don't need to hard-code the
	// raw string.
	GenAIOperationNameInvokeWorkflow = "invoke_workflow"
)

// targetTypes maps each gen_ai.* key with an enforced Go type to that type.
// Keys whose spec type is "any" (e.g. gen_ai.input.messages) or an open enum
// not coerced (e.g. gen_ai.operation.name) are not registered; Coerce treats
// absent keys as opaque and passes values through.
//
//nolint:gochecknoglobals // canonical type registry
var targetTypes = map[string]reflect.Type{
	GenAIAgentName:               reflect.TypeFor[string](),
	GenAIConversationID:          reflect.TypeFor[string](),
	GenAIRequestFrequencyPenalty: reflect.TypeFor[float64](),
	GenAIRequestMaxTokens:        reflect.TypeFor[int](),
	GenAIRequestModel:            reflect.TypeFor[string](),
	GenAIRequestPresencePenalty:  reflect.TypeFor[float64](),
	GenAIRequestStopSequences:    reflect.TypeFor[[]string](),
	GenAIRequestTemperature:      reflect.TypeFor[float64](),
	GenAIRequestTopK:             reflect.TypeFor[float64](),
	GenAIRequestTopP:             reflect.TypeFor[float64](),
	GenAIResponseFinishReasons:   reflect.TypeFor[[]string](),
	GenAIResponseModel:           reflect.TypeFor[string](),
	GenAIToolCallID:              reflect.TypeFor[string](),
	GenAIToolDescription:         reflect.TypeFor[string](),
	GenAIToolName:                reflect.TypeFor[string](),
	GenAIUsageInputTokens:        reflect.TypeFor[int](),
	GenAIUsageOutputTokens:       reflect.TypeFor[int](),
}
